#!/usr/bin/env python3
"""
Benchmark Report Generator for Database Intelligence System

Generates comprehensive performance reports from benchmark results including:
- Performance comparisons
- Trend analysis
- Resource usage charts
- Regression detection
- HTML and Markdown reports
"""

import json
import os
import sys
from pathlib import Path
from typing import Dict, List, Any, Optional
from datetime import datetime
import statistics
import html
from dataclasses import dataclass
import matplotlib.pyplot as plt
import matplotlib.dates as mdates
from matplotlib.figure import Figure
import numpy as np


@dataclass
class BenchmarkComparison:
    """Comparison between two benchmark runs"""
    name: str
    baseline_value: float
    current_value: float
    difference: float
    percent_change: float
    regression: bool
    
    
class ReportGenerator:
    """Generate benchmark reports in various formats"""
    
    def __init__(self, results_dir: str = "."):
        self.results_dir = Path(results_dir)
        self.results = []
        self.comparisons = []
        
    def load_results(self, pattern: str = "*benchmark_results.json"):
        """Load benchmark results from JSON files"""
        self.results = []
        
        for file_path in self.results_dir.glob(pattern):
            with open(file_path, 'r') as f:
                data = json.load(f)
                data['file'] = file_path.name
                self.results.append(data)
                
        # Sort by timestamp
        self.results.sort(key=lambda x: x.get('timestamp', ''))
        
        print(f"Loaded {len(self.results)} benchmark results")
        
    def compare_results(self, baseline_file: str, current_file: str) -> List[BenchmarkComparison]:
        """Compare two benchmark result files"""
        baseline = None
        current = None
        
        # Find the files
        for result in self.results:
            if result['file'] == baseline_file:
                baseline = result
            elif result['file'] == current_file:
                current = result
                
        if not baseline or not current:
            raise ValueError(f"Could not find specified files: {baseline_file}, {current_file}")
            
        comparisons = []
        
        # Create lookup for baseline results
        baseline_lookup = {}
        for result in baseline['results']:
            key = f"{result['category']}_{result['name']}"
            baseline_lookup[key] = result
            
        # Compare each current result with baseline
        for result in current['results']:
            key = f"{result['category']}_{result['name']}"
            
            if key in baseline_lookup:
                baseline_result = baseline_lookup[key]
                
                # Compare average duration
                baseline_avg = baseline_result['avg_duration']
                current_avg = result['avg_duration']
                difference = current_avg - baseline_avg
                percent_change = (difference / baseline_avg) * 100 if baseline_avg > 0 else 0
                
                # Regression if performance degraded by more than 10%
                regression = percent_change > 10
                
                comparisons.append(BenchmarkComparison(
                    name=result['name'],
                    baseline_value=baseline_avg,
                    current_value=current_avg,
                    difference=difference,
                    percent_change=percent_change,
                    regression=regression
                ))
                
        self.comparisons = comparisons
        return comparisons
        
    def generate_trend_charts(self, output_dir: str = "charts"):
        """Generate trend charts for benchmark results over time"""
        output_path = Path(output_dir)
        output_path.mkdir(exist_ok=True)
        
        # Group results by benchmark name
        benchmarks = {}
        for result_set in self.results:
            timestamp = datetime.fromisoformat(result_set['timestamp'])
            
            for result in result_set['results']:
                name = result['name']
                if name not in benchmarks:
                    benchmarks[name] = {'timestamps': [], 'durations': [], 'cpu': [], 'memory': []}
                    
                benchmarks[name]['timestamps'].append(timestamp)
                benchmarks[name]['durations'].append(result['avg_duration'] * 1000)  # Convert to ms
                
                # Extract resource metrics if available
                if 'resources' in result.get('metrics', {}):
                    resources = result['metrics']['resources']
                    benchmarks[name]['cpu'].append(resources.get('cpu', {}).get('avg', 0))
                    benchmarks[name]['memory'].append(resources.get('memory', {}).get('avg', 0))
                    
        # Generate charts for each benchmark
        for name, data in benchmarks.items():
            if len(data['timestamps']) < 2:
                continue
                
            fig, axes = plt.subplots(3, 1, figsize=(10, 12))
            fig.suptitle(f'Benchmark Trend: {name}', fontsize=16)
            
            # Duration trend
            ax1 = axes[0]
            ax1.plot(data['timestamps'], data['durations'], 'b-o')
            ax1.set_ylabel('Duration (ms)')
            ax1.set_title('Execution Time Trend')
            ax1.grid(True)
            ax1.xaxis.set_major_formatter(mdates.DateFormatter('%Y-%m-%d %H:%M'))
            
            # CPU usage trend
            if data['cpu']:
                ax2 = axes[1]
                ax2.plot(data['timestamps'], data['cpu'], 'r-o')
                ax2.set_ylabel('CPU Usage (%)')
                ax2.set_title('CPU Usage Trend')
                ax2.grid(True)
                ax2.xaxis.set_major_formatter(mdates.DateFormatter('%Y-%m-%d %H:%M'))
                
            # Memory usage trend
            if data['memory']:
                ax3 = axes[2]
                ax3.plot(data['timestamps'], data['memory'], 'g-o')
                ax3.set_ylabel('Memory Usage (MB)')
                ax3.set_title('Memory Usage Trend')
                ax3.grid(True)
                ax3.xaxis.set_major_formatter(mdates.DateFormatter('%Y-%m-%d %H:%M'))
                
            # Rotate x-axis labels
            for ax in axes:
                ax.tick_params(axis='x', rotation=45)
                
            plt.tight_layout()
            
            # Save chart
            safe_name = name.replace(' ', '_').replace('/', '_')
            plt.savefig(output_path / f'trend_{safe_name}.png', dpi=150, bbox_inches='tight')
            plt.close()
            
        print(f"Generated trend charts in {output_path}")
        
    def generate_comparison_chart(self, output_file: str = "comparison.png"):
        """Generate comparison chart between baseline and current results"""
        if not self.comparisons:
            print("No comparisons to chart")
            return
            
        # Sort by percent change
        self.comparisons.sort(key=lambda x: x.percent_change, reverse=True)
        
        # Prepare data
        names = [c.name for c in self.comparisons[:20]]  # Top 20
        baseline_values = [c.baseline_value * 1000 for c in self.comparisons[:20]]
        current_values = [c.current_value * 1000 for c in self.comparisons[:20]]
        
        # Create figure
        fig, ax = plt.subplots(figsize=(12, 8))
        
        x = np.arange(len(names))
        width = 0.35
        
        # Create bars
        bars1 = ax.bar(x - width/2, baseline_values, width, label='Baseline', color='blue', alpha=0.7)
        bars2 = ax.bar(x + width/2, current_values, width, label='Current', color='red', alpha=0.7)
        
        # Add percentage change labels
        for i, comp in enumerate(self.comparisons[:20]):
            color = 'red' if comp.regression else 'green'
            ax.text(i, max(baseline_values[i], current_values[i]) + 1,
                   f'{comp.percent_change:+.1f}%', ha='center', color=color, fontsize=9)
                   
        ax.set_xlabel('Benchmark')
        ax.set_ylabel('Duration (ms)')
        ax.set_title('Benchmark Performance Comparison')
        ax.set_xticks(x)
        ax.set_xticklabels(names, rotation=45, ha='right')
        ax.legend()
        ax.grid(True, axis='y', alpha=0.3)
        
        plt.tight_layout()
        plt.savefig(output_file, dpi=150, bbox_inches='tight')
        plt.close()
        
        print(f"Generated comparison chart: {output_file}")
        
    def generate_html_report(self, output_file: str = "benchmark_report.html"):
        """Generate comprehensive HTML report"""
        latest_result = self.results[-1] if self.results else None
        
        html_content = f"""
<!DOCTYPE html>
<html>
<head>
    <title>Database Intelligence Benchmark Report</title>
    <style>
        body {{
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }}
        .container {{
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            padding: 20px;
            box-shadow: 0 0 10px rgba(0,0,0,0.1);
        }}
        h1, h2, h3 {{
            color: #333;
        }}
        .summary {{
            background-color: #e8f4f8;
            padding: 15px;
            border-radius: 5px;
            margin-bottom: 20px;
        }}
        .metric {{
            display: inline-block;
            margin: 10px 20px;
        }}
        .metric-value {{
            font-size: 24px;
            font-weight: bold;
            color: #2196F3;
        }}
        .metric-label {{
            font-size: 14px;
            color: #666;
        }}
        table {{
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }}
        th, td {{
            padding: 10px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }}
        th {{
            background-color: #f8f8f8;
            font-weight: bold;
        }}
        tr:hover {{
            background-color: #f5f5f5;
        }}
        .regression {{
            color: #d32f2f;
            font-weight: bold;
        }}
        .improvement {{
            color: #388e3c;
            font-weight: bold;
        }}
        .chart {{
            margin: 20px 0;
            text-align: center;
        }}
        .chart img {{
            max-width: 100%;
            border: 1px solid #ddd;
            border-radius: 5px;
        }}
        .section {{
            margin-bottom: 40px;
        }}
        .timestamp {{
            color: #666;
            font-size: 14px;
        }}
        .error {{
            background-color: #ffebee;
            color: #c62828;
            padding: 10px;
            border-radius: 5px;
            margin: 10px 0;
        }}
    </style>
</head>
<body>
    <div class="container">
        <h1>Database Intelligence Benchmark Report</h1>
        <p class="timestamp">Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>
"""
        
        # Summary section
        if latest_result:
            total_benchmarks = len(latest_result['results'])
            total_errors = sum(1 for r in latest_result['results'] if r.get('errors'))
            avg_duration = statistics.mean(r['avg_duration'] for r in latest_result['results'])
            
            html_content += f"""
        <div class="section">
            <h2>Latest Run Summary</h2>
            <div class="summary">
                <div class="metric">
                    <div class="metric-value">{total_benchmarks}</div>
                    <div class="metric-label">Total Benchmarks</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{total_errors}</div>
                    <div class="metric-label">Failed Tests</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{avg_duration*1000:.2f}ms</div>
                    <div class="metric-label">Average Duration</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{latest_result['timestamp']}</div>
                    <div class="metric-label">Timestamp</div>
                </div>
            </div>
        </div>
"""
            
        # Comparison section
        if self.comparisons:
            regressions = [c for c in self.comparisons if c.regression]
            improvements = [c for c in self.comparisons if c.percent_change < -10]
            
            html_content += f"""
        <div class="section">
            <h2>Performance Comparison</h2>
            <div class="summary">
                <div class="metric">
                    <div class="metric-value regression">{len(regressions)}</div>
                    <div class="metric-label">Regressions</div>
                </div>
                <div class="metric">
                    <div class="metric-value improvement">{len(improvements)}</div>
                    <div class="metric-label">Improvements</div>
                </div>
            </div>
            
            <div class="chart">
                <img src="comparison.png" alt="Performance Comparison Chart">
            </div>
            
            <h3>Detailed Comparison</h3>
            <table>
                <tr>
                    <th>Benchmark</th>
                    <th>Baseline (ms)</th>
                    <th>Current (ms)</th>
                    <th>Difference (ms)</th>
                    <th>Change (%)</th>
                    <th>Status</th>
                </tr>
"""
            
            for comp in sorted(self.comparisons, key=lambda x: x.percent_change, reverse=True):
                status_class = 'regression' if comp.regression else ('improvement' if comp.percent_change < -10 else '')
                status_text = 'Regression' if comp.regression else ('Improvement' if comp.percent_change < -10 else 'OK')
                
                html_content += f"""
                <tr>
                    <td>{html.escape(comp.name)}</td>
                    <td>{comp.baseline_value*1000:.2f}</td>
                    <td>{comp.current_value*1000:.2f}</td>
                    <td>{comp.difference*1000:.2f}</td>
                    <td class="{status_class}">{comp.percent_change:+.1f}%</td>
                    <td class="{status_class}">{status_text}</td>
                </tr>
"""
                
            html_content += """
            </table>
        </div>
"""
            
        # Detailed results section
        if latest_result:
            html_content += """
        <div class="section">
            <h2>Detailed Benchmark Results</h2>
            <table>
                <tr>
                    <th>Category</th>
                    <th>Benchmark</th>
                    <th>Iterations</th>
                    <th>Avg Duration (ms)</th>
                    <th>Min (ms)</th>
                    <th>Max (ms)</th>
                    <th>P95 (ms)</th>
                    <th>CPU Avg (%)</th>
                    <th>Memory Avg (MB)</th>
                </tr>
"""
            
            for result in sorted(latest_result['results'], key=lambda x: x['category']):
                timings = result.get('metrics', {}).get('timings', {})
                resources = result.get('metrics', {}).get('resources', {})
                
                error_indicator = ' ⚠️' if result.get('errors') else ''
                
                html_content += f"""
                <tr>
                    <td>{html.escape(result['category'])}</td>
                    <td>{html.escape(result['name'])}{error_indicator}</td>
                    <td>{result['iterations']}</td>
                    <td>{result['avg_duration']*1000:.2f}</td>
                    <td>{timings.get('min', 0)*1000:.2f}</td>
                    <td>{timings.get('max', 0)*1000:.2f}</td>
                    <td>{timings.get('p95', 0)*1000:.2f}</td>
                    <td>{resources.get('cpu', {}).get('avg', 0):.1f}</td>
                    <td>{resources.get('memory', {}).get('avg', 0):.1f}</td>
                </tr>
"""
                
                # Show errors if any
                if result.get('errors'):
                    html_content += f"""
                <tr>
                    <td colspan="9" class="error">
                        Errors: {html.escape(', '.join(result['errors']))}
                    </td>
                </tr>
"""
                    
            html_content += """
            </table>
        </div>
"""
            
        # Trend charts section
        html_content += """
        <div class="section">
            <h2>Performance Trends</h2>
            <p>Historical performance trends for key benchmarks:</p>
"""
        
        # Add trend chart images
        charts_dir = Path("charts")
        if charts_dir.exists():
            for chart_file in sorted(charts_dir.glob("trend_*.png")):
                html_content += f"""
            <div class="chart">
                <img src="charts/{chart_file.name}" alt="{chart_file.stem}">
            </div>
"""
                
        html_content += """
        </div>
    </div>
</body>
</html>
"""
        
        with open(output_file, 'w') as f:
            f.write(html_content)
            
        print(f"Generated HTML report: {output_file}")
        
    def generate_markdown_report(self, output_file: str = "benchmark_report.md"):
        """Generate Markdown report"""
        latest_result = self.results[-1] if self.results else None
        
        md_content = f"""# Database Intelligence Benchmark Report

Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}

## Executive Summary

"""
        
        if latest_result:
            total_benchmarks = len(latest_result['results'])
            total_errors = sum(1 for r in latest_result['results'] if r.get('errors'))
            avg_duration = statistics.mean(r['avg_duration'] for r in latest_result['results'])
            
            md_content += f"""- **Total Benchmarks**: {total_benchmarks}
- **Failed Tests**: {total_errors}
- **Average Duration**: {avg_duration*1000:.2f}ms
- **Run Timestamp**: {latest_result['timestamp']}

"""
            
        if self.comparisons:
            regressions = [c for c in self.comparisons if c.regression]
            improvements = [c for c in self.comparisons if c.percent_change < -10]
            
            md_content += f"""## Performance Comparison

- **Regressions Detected**: {len(regressions)}
- **Significant Improvements**: {len(improvements)}

### Top Regressions

| Benchmark | Baseline (ms) | Current (ms) | Change (%) |
|-----------|---------------|--------------|------------|
"""
            
            for comp in sorted(regressions, key=lambda x: x.percent_change, reverse=True)[:10]:
                md_content += f"| {comp.name} | {comp.baseline_value*1000:.2f} | {comp.current_value*1000:.2f} | **+{comp.percent_change:.1f}%** |\n"
                
            md_content += """
### Top Improvements

| Benchmark | Baseline (ms) | Current (ms) | Change (%) |
|-----------|---------------|--------------|------------|
"""
            
            for comp in sorted(improvements, key=lambda x: x.percent_change)[:10]:
                md_content += f"| {comp.name} | {comp.baseline_value*1000:.2f} | {comp.current_value*1000:.2f} | **{comp.percent_change:.1f}%** |\n"
                
        if latest_result:
            md_content += """
## Detailed Results by Category

"""
            
            # Group by category
            categories = {}
            for result in latest_result['results']:
                cat = result['category']
                if cat not in categories:
                    categories[cat] = []
                categories[cat].append(result)
                
            for category, results in sorted(categories.items()):
                md_content += f"""### {category.title().replace('_', ' ')}

| Benchmark | Iterations | Avg (ms) | Min (ms) | Max (ms) | P95 (ms) |
|-----------|------------|----------|----------|----------|----------|
"""
                
                for result in sorted(results, key=lambda x: x['avg_duration'], reverse=True):
                    timings = result.get('metrics', {}).get('timings', {})
                    error_indicator = ' ⚠️' if result.get('errors') else ''
                    
                    md_content += f"| {result['name']}{error_indicator} | "
                    md_content += f"{result['iterations']} | "
                    md_content += f"{result['avg_duration']*1000:.2f} | "
                    md_content += f"{timings.get('min', 0)*1000:.2f} | "
                    md_content += f"{timings.get('max', 0)*1000:.2f} | "
                    md_content += f"{timings.get('p95', 0)*1000:.2f} |\n"
                    
        md_content += """
## System Information

"""
        
        if latest_result and 'system_info' in latest_result:
            info = latest_result['system_info']
            md_content += f"""- **Platform**: {info.get('platform', 'Unknown')}
- **CPU Count**: {info.get('cpu_count', 'Unknown')}
- **Memory**: {info.get('memory_gb', 0):.1f} GB

"""
            
        md_content += """## Recommendations

Based on the benchmark results:

"""
        
        if self.comparisons:
            if regressions:
                md_content += """1. **Performance Regressions**: Several benchmarks show performance degradation. Investigate:
"""
                for comp in regressions[:5]:
                    md_content += f"   - {comp.name}: {comp.percent_change:+.1f}% slower\n"
                    
            if improvements:
                md_content += """\n2. **Performance Improvements**: Some areas show significant improvement:
"""
                for comp in improvements[:5]:
                    md_content += f"   - {comp.name}: {comp.percent_change:.1f}% faster\n"
                    
        with open(output_file, 'w') as f:
            f.write(md_content)
            
        print(f"Generated Markdown report: {output_file}")
        
    def generate_all_reports(self):
        """Generate all report types"""
        if not self.results:
            print("No results loaded. Run load_results() first.")
            return
            
        print("\nGenerating reports...")
        
        # Generate trend charts
        self.generate_trend_charts()
        
        # If we have at least 2 results, compare the last two
        if len(self.results) >= 2:
            baseline_file = self.results[-2]['file']
            current_file = self.results[-1]['file']
            
            print(f"\nComparing {baseline_file} vs {current_file}")
            self.compare_results(baseline_file, current_file)
            self.generate_comparison_chart()
            
        # Generate reports
        self.generate_html_report()
        self.generate_markdown_report()
        
        print("\nReport generation complete!")


def main():
    """Main entry point"""
    import argparse
    
    parser = argparse.ArgumentParser(description="Generate benchmark reports")
    parser.add_argument("--results-dir", default=".", help="Directory containing benchmark results")
    parser.add_argument("--compare", nargs=2, metavar=('BASELINE', 'CURRENT'),
                        help="Compare two specific result files")
    parser.add_argument("--format", choices=['html', 'markdown', 'all'], default='all',
                        help="Report format to generate")
    
    args = parser.parse_args()
    
    # Create report generator
    generator = ReportGenerator(args.results_dir)
    
    # Load results
    generator.load_results()
    
    if not generator.results:
        print("No benchmark results found!")
        return
        
    # Handle comparison
    if args.compare:
        generator.compare_results(args.compare[0], args.compare[1])
        generator.generate_comparison_chart()
        
    # Generate reports based on format
    if args.format == 'html' or args.format == 'all':
        generator.generate_html_report()
    if args.format == 'markdown' or args.format == 'all':
        generator.generate_markdown_report()
    if args.format == 'all':
        generator.generate_trend_charts()


if __name__ == "__main__":
    main()