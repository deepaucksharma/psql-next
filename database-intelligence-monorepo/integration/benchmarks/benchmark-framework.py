#!/usr/bin/env python3
"""
Performance Benchmarking Framework for Database Intelligence System

This framework provides comprehensive performance measurement capabilities
for all components of the database intelligence monitoring system.
"""

import time
import json
import psutil
import threading
import statistics
from datetime import datetime
from typing import Dict, List, Any, Callable, Optional
from dataclasses import dataclass, field
from contextlib import contextmanager
import subprocess
import os
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.append(str(Path(__file__).parent.parent.parent))


@dataclass
class BenchmarkResult:
    """Container for benchmark results"""
    name: str
    category: str
    start_time: float
    end_time: float
    duration: float
    iterations: int
    metrics: Dict[str, Any] = field(default_factory=dict)
    errors: List[str] = field(default_factory=list)
    metadata: Dict[str, Any] = field(default_factory=dict)
    
    @property
    def avg_duration(self) -> float:
        """Average duration per iteration"""
        return self.duration / self.iterations if self.iterations > 0 else 0
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for serialization"""
        return {
            'name': self.name,
            'category': self.category,
            'start_time': self.start_time,
            'end_time': self.end_time,
            'duration': self.duration,
            'iterations': self.iterations,
            'avg_duration': self.avg_duration,
            'metrics': self.metrics,
            'errors': self.errors,
            'metadata': self.metadata
        }


class ResourceMonitor:
    """Monitor system resource usage during benchmarks"""
    
    def __init__(self, pid: Optional[int] = None):
        self.pid = pid or os.getpid()
        self.process = psutil.Process(self.pid)
        self.monitoring = False
        self.samples = []
        self._monitor_thread = None
        
    def start(self):
        """Start resource monitoring"""
        self.monitoring = True
        self.samples = []
        self._monitor_thread = threading.Thread(target=self._monitor_loop)
        self._monitor_thread.daemon = True
        self._monitor_thread.start()
        
    def stop(self) -> Dict[str, Any]:
        """Stop monitoring and return statistics"""
        self.monitoring = False
        if self._monitor_thread:
            self._monitor_thread.join()
            
        if not self.samples:
            return {}
            
        # Calculate statistics
        cpu_samples = [s['cpu_percent'] for s in self.samples]
        memory_samples = [s['memory_mb'] for s in self.samples]
        
        return {
            'cpu': {
                'min': min(cpu_samples),
                'max': max(cpu_samples),
                'avg': statistics.mean(cpu_samples),
                'median': statistics.median(cpu_samples),
                'samples': len(cpu_samples)
            },
            'memory': {
                'min': min(memory_samples),
                'max': max(memory_samples),
                'avg': statistics.mean(memory_samples),
                'median': statistics.median(memory_samples),
                'samples': len(memory_samples)
            }
        }
        
    def _monitor_loop(self):
        """Background monitoring loop"""
        while self.monitoring:
            try:
                with self.process.oneshot():
                    self.samples.append({
                        'timestamp': time.time(),
                        'cpu_percent': self.process.cpu_percent(),
                        'memory_mb': self.process.memory_info().rss / 1024 / 1024
                    })
            except psutil.NoSuchProcess:
                break
            time.sleep(0.1)  # Sample every 100ms


class BenchmarkFramework:
    """Main benchmarking framework"""
    
    def __init__(self, name: str = "Database Intelligence Benchmarks"):
        self.name = name
        self.results: List[BenchmarkResult] = []
        self.resource_monitor = ResourceMonitor()
        
    @contextmanager
    def benchmark(self, name: str, category: str, iterations: int = 1, 
                  warmup_iterations: int = 0, metadata: Dict[str, Any] = None):
        """
        Context manager for benchmarking a code block
        
        Args:
            name: Benchmark name
            category: Category (e.g., 'startup', 'collection', 'query')
            iterations: Number of iterations to run
            warmup_iterations: Number of warmup iterations before timing
            metadata: Additional metadata to store
        """
        # Run warmup iterations
        if warmup_iterations > 0:
            print(f"Running {warmup_iterations} warmup iterations for {name}...")
            
        result = BenchmarkResult(
            name=name,
            category=category,
            start_time=0,
            end_time=0,
            duration=0,
            iterations=iterations,
            metadata=metadata or {}
        )
        
        # Start resource monitoring
        self.resource_monitor.start()
        
        try:
            # Record start time
            result.start_time = time.time()
            
            yield result
            
            # Record end time
            result.end_time = time.time()
            result.duration = result.end_time - result.start_time
            
        except Exception as e:
            result.errors.append(str(e))
            raise
            
        finally:
            # Stop resource monitoring and collect stats
            resource_stats = self.resource_monitor.stop()
            result.metrics['resources'] = resource_stats
            
            # Store result
            self.results.append(result)
            
            # Print summary
            print(f"\nBenchmark: {name}")
            print(f"  Duration: {result.duration:.3f}s ({result.avg_duration:.3f}s per iteration)")
            if resource_stats:
                print(f"  CPU: {resource_stats['cpu']['avg']:.1f}% avg")
                print(f"  Memory: {resource_stats['memory']['avg']:.1f}MB avg")
    
    def time_function(self, func: Callable, name: str, category: str,
                      iterations: int = 100, warmup_iterations: int = 10,
                      args: tuple = (), kwargs: dict = None) -> BenchmarkResult:
        """
        Benchmark a single function
        
        Args:
            func: Function to benchmark
            name: Benchmark name
            category: Category
            iterations: Number of iterations
            warmup_iterations: Warmup iterations
            args: Function arguments
            kwargs: Function keyword arguments
        """
        kwargs = kwargs or {}
        
        # Warmup
        for _ in range(warmup_iterations):
            func(*args, **kwargs)
            
        # Benchmark
        with self.benchmark(name, category, iterations) as result:
            timings = []
            for _ in range(iterations):
                start = time.perf_counter()
                func(*args, **kwargs)
                end = time.perf_counter()
                timings.append(end - start)
                
            result.metrics['timings'] = {
                'min': min(timings),
                'max': max(timings),
                'avg': statistics.mean(timings),
                'median': statistics.median(timings),
                'stdev': statistics.stdev(timings) if len(timings) > 1 else 0,
                'p95': sorted(timings)[int(len(timings) * 0.95)],
                'p99': sorted(timings)[int(len(timings) * 0.99)]
            }
            
        return result
    
    def benchmark_subprocess(self, command: List[str], name: str, category: str,
                           iterations: int = 1, env: Dict[str, str] = None) -> BenchmarkResult:
        """
        Benchmark a subprocess command
        
        Args:
            command: Command to run
            name: Benchmark name
            category: Category
            iterations: Number of iterations
            env: Environment variables
        """
        with self.benchmark(name, category, iterations) as result:
            startup_times = []
            
            for _ in range(iterations):
                start = time.perf_counter()
                proc = subprocess.Popen(
                    command,
                    env={**os.environ, **(env or {})},
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE
                )
                
                # Monitor subprocess resources
                if proc.pid:
                    sub_monitor = ResourceMonitor(proc.pid)
                    sub_monitor.start()
                    
                stdout, stderr = proc.communicate()
                end = time.perf_counter()
                
                startup_times.append(end - start)
                
                if proc.returncode != 0:
                    result.errors.append(f"Process failed: {stderr.decode()}")
                    
                if proc.pid:
                    result.metrics[f'subprocess_{_}'] = sub_monitor.stop()
                    
            result.metrics['startup_times'] = {
                'min': min(startup_times),
                'max': max(startup_times),
                'avg': statistics.mean(startup_times),
                'median': statistics.median(startup_times)
            }
            
        return result
    
    def compare_results(self, baseline: BenchmarkResult, current: BenchmarkResult) -> Dict[str, Any]:
        """Compare two benchmark results"""
        comparison = {
            'name': current.name,
            'baseline_duration': baseline.avg_duration,
            'current_duration': current.avg_duration,
            'difference': current.avg_duration - baseline.avg_duration,
            'percent_change': ((current.avg_duration - baseline.avg_duration) / baseline.avg_duration) * 100
        }
        
        # Compare resource usage
        if 'resources' in baseline.metrics and 'resources' in current.metrics:
            baseline_cpu = baseline.metrics['resources']['cpu']['avg']
            current_cpu = current.metrics['resources']['cpu']['avg']
            baseline_mem = baseline.metrics['resources']['memory']['avg']
            current_mem = current.metrics['resources']['memory']['avg']
            
            comparison['resources'] = {
                'cpu': {
                    'baseline': baseline_cpu,
                    'current': current_cpu,
                    'percent_change': ((current_cpu - baseline_cpu) / baseline_cpu) * 100
                },
                'memory': {
                    'baseline': baseline_mem,
                    'current': current_mem,
                    'percent_change': ((current_mem - baseline_mem) / baseline_mem) * 100
                }
            }
            
        return comparison
    
    def save_results(self, filename: str):
        """Save benchmark results to JSON file"""
        data = {
            'name': self.name,
            'timestamp': datetime.now().isoformat(),
            'system_info': {
                'platform': os.uname().sysname,
                'cpu_count': psutil.cpu_count(),
                'memory_gb': psutil.virtual_memory().total / 1024 / 1024 / 1024
            },
            'results': [r.to_dict() for r in self.results]
        }
        
        with open(filename, 'w') as f:
            json.dump(data, f, indent=2)
            
        print(f"\nResults saved to {filename}")
    
    def print_summary(self):
        """Print summary of all benchmarks"""
        print(f"\n{'='*60}")
        print(f"Benchmark Summary: {self.name}")
        print(f"{'='*60}\n")
        
        # Group by category
        categories = {}
        for result in self.results:
            if result.category not in categories:
                categories[result.category] = []
            categories[result.category].append(result)
            
        for category, results in categories.items():
            print(f"\n{category.upper()}")
            print("-" * 40)
            
            for result in results:
                status = "✓" if not result.errors else "✗"
                print(f"{status} {result.name:<30} {result.avg_duration*1000:>8.2f}ms")
                
                if result.errors:
                    for error in result.errors:
                        print(f"  ERROR: {error}")
                        
                if 'timings' in result.metrics:
                    timings = result.metrics['timings']
                    print(f"  p50: {timings['median']*1000:.2f}ms  "
                          f"p95: {timings['p95']*1000:.2f}ms  "
                          f"p99: {timings['p99']*1000:.2f}ms")


if __name__ == "__main__":
    # Example usage
    framework = BenchmarkFramework("Example Benchmarks")
    
    # Benchmark a simple function
    def example_function(n=1000):
        return sum(range(n))
    
    framework.time_function(
        example_function,
        name="Sum calculation",
        category="computation",
        iterations=1000,
        kwargs={'n': 10000}
    )
    
    # Benchmark with context manager
    with framework.benchmark("Sleep test", "timing", iterations=5) as result:
        for i in range(result.iterations):
            time.sleep(0.1)
            
    framework.print_summary()
    framework.save_results("example_benchmark_results.json")