#!/usr/bin/env python3
"""
Baseline calculator for anomaly detection
This would be used in a production system to calculate dynamic baselines
"""

import numpy as np
from collections import deque
from datetime import datetime, timedelta


class BaselineCalculator:
    """Calculate rolling baselines for metrics"""
    
    def __init__(self, window_size=100, seasonality_period=24):
        self.window_size = window_size
        self.seasonality_period = seasonality_period
        self.values = deque(maxlen=window_size)
        self.timestamps = deque(maxlen=window_size)
    
    def add_value(self, value, timestamp=None):
        """Add a new value to the baseline calculation"""
        if timestamp is None:
            timestamp = datetime.now()
        
        self.values.append(value)
        self.timestamps.append(timestamp)
    
    def get_baseline_stats(self):
        """Calculate baseline mean and standard deviation"""
        if len(self.values) < 2:
            return None, None
        
        values_array = np.array(self.values)
        
        # Remove outliers using IQR method
        q1 = np.percentile(values_array, 25)
        q3 = np.percentile(values_array, 75)
        iqr = q3 - q1
        lower_bound = q1 - 1.5 * iqr
        upper_bound = q3 + 1.5 * iqr
        
        filtered_values = values_array[
            (values_array >= lower_bound) & (values_array <= upper_bound)
        ]
        
        if len(filtered_values) < 2:
            return np.mean(values_array), np.std(values_array)
        
        return np.mean(filtered_values), np.std(filtered_values)
    
    def calculate_zscore(self, current_value):
        """Calculate z-score for current value"""
        mean, stddev = self.get_baseline_stats()
        
        if mean is None or stddev is None or stddev == 0:
            return 0
        
        return (current_value - mean) / stddev
    
    def detect_trend(self):
        """Detect if there's a trend in the baseline"""
        if len(self.values) < 10:
            return "stable"
        
        recent_values = list(self.values)[-10:]
        older_values = list(self.values)[-20:-10] if len(self.values) >= 20 else list(self.values)[:-10]
        
        recent_mean = np.mean(recent_values)
        older_mean = np.mean(older_values)
        
        if recent_mean > older_mean * 1.1:
            return "increasing"
        elif recent_mean < older_mean * 0.9:
            return "decreasing"
        else:
            return "stable"


class SeasonalBaselineCalculator(BaselineCalculator):
    """Baseline calculator with seasonality adjustment"""
    
    def __init__(self, window_size=168, seasonality_period=24):  # 1 week of hourly data
        super().__init__(window_size, seasonality_period)
        self.seasonal_baselines = {}
    
    def get_seasonal_baseline(self, timestamp):
        """Get baseline for specific time period"""
        hour_of_day = timestamp.hour
        day_of_week = timestamp.weekday()
        
        key = f"{day_of_week}_{hour_of_day}"
        
        if key not in self.seasonal_baselines:
            return self.get_baseline_stats()
        
        return self.seasonal_baselines[key]
    
    def update_seasonal_baselines(self):
        """Update seasonal baseline calculations"""
        if len(self.values) < self.seasonality_period * 2:
            return
        
        # Group values by time period
        for i, (value, timestamp) in enumerate(zip(self.values, self.timestamps)):
            hour_of_day = timestamp.hour
            day_of_week = timestamp.weekday()
            key = f"{day_of_week}_{hour_of_day}"
            
            if key not in self.seasonal_baselines:
                self.seasonal_baselines[key] = {
                    'values': [],
                    'mean': 0,
                    'stddev': 0
                }
            
            self.seasonal_baselines[key]['values'].append(value)
        
        # Calculate stats for each period
        for key, data in self.seasonal_baselines.items():
            if len(data['values']) >= 2:
                data['mean'] = np.mean(data['values'])
                data['stddev'] = np.std(data['values'])


if __name__ == "__main__":
    # Example usage
    calculator = BaselineCalculator()
    
    # Simulate normal values
    for i in range(50):
        calculator.add_value(100 + np.random.normal(0, 5))
    
    # Add anomaly
    anomaly_value = 150
    calculator.add_value(anomaly_value)
    
    zscore = calculator.calculate_zscore(anomaly_value)
    print(f"Z-score for anomaly: {zscore:.2f}")
    print(f"Trend: {calculator.detect_trend()}")