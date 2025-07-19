# Database Intelligence Shared Dashboards

This directory contains shared Grafana dashboard components and a generator script for creating module-specific dashboards.

## Structure

```
shared/dashboards/
├── base-dashboard.json      # Base dashboard template with reusable panels
├── dashboard-generator.py   # Script to generate module-specific dashboards
└── README.md               # This file
```

## Base Dashboard Template

The `base-dashboard.json` file contains:

### Common Template Variables
- **datasource**: Prometheus data source selector
- **interval**: Time interval for rate calculations (5m, 10m, 30m, 1h)
- **instance**: Instance filter for multi-instance deployments

### Reusable Panel Library
The base dashboard includes a `panels_library` section with pre-configured panels:

1. **cpu_usage**: CPU utilization percentage
2. **memory_usage**: Memory utilization percentage
3. **query_rate**: Database query rate per second
4. **connections**: Current database connections gauge
5. **error_rate**: Connection error rate percentage

Each panel in the library includes:
- Complete Grafana panel configuration
- Prometheus queries with variable substitution
- Consistent styling and thresholds
- Proper units and formatting

## Dashboard Generator

The `dashboard-generator.py` script provides a programmatic way to create module-specific dashboards.

### Features
- Reuse panels from the base dashboard library
- Add custom panels specific to each module
- Automatic grid positioning
- Custom template variables per module
- Consistent styling and tagging

### Usage

Generate a specific module dashboard:
```bash
python dashboard-generator.py anomaly-detector -o ./output
```

Generate all module dashboards:
```bash
python dashboard-generator.py all -o ./output
```

### Command Line Options
- `module`: Module to generate dashboard for (anomaly-detector, performance-tuner, query-optimizer, all)
- `-o, --output-dir`: Output directory for generated dashboards (default: current directory)
- `-b, --base-dashboard`: Path to base dashboard template (default: base-dashboard.json)

### Creating Custom Dashboards

To create a dashboard for a new module:

1. Add a new function in `dashboard-generator.py`:
```python
def create_my_module_dashboard():
    generator = DashboardGenerator()
    
    panels = [
        {'panel_ref': 'cpu_usage'},  # Reuse from library
        {
            'panel_ref': 'query_rate',
            'overrides': {
                'title': 'Custom Query Rate',
                'targets': [...]  # Custom queries
            }
        },
        {
            # Complete custom panel definition
            'datasource': {...},
            'fieldConfig': {...},
            'options': {...},
            'targets': [...],
            'title': 'My Custom Panel',
            'type': 'timeseries'
        }
    ]
    
    dashboard = generator.generate_dashboard(
        module_name='my-module',
        title='My Module Dashboard',
        description='Description of my module dashboard',
        panels=panels,
        tags=['my-module', 'custom']
    )
    
    return dashboard
```

2. Register the function in the `main()` function's generators dictionary

## Module-Specific Dashboards

### Anomaly Detector Dashboard
- Includes all base panels
- Enhanced query rate panel with anomaly thresholds
- Anomaly count statistics
- Custom variable for filtering by anomaly type

### Performance Tuner Dashboard
- System resource panels (CPU, Memory)
- Query performance analysis
- Top 10 slowest queries pie chart
- Performance recommendations table

### Query Optimizer Dashboard
- Query rate monitoring
- Query optimization scores over time
- Query execution plans table
- Detailed query analysis metrics

## Importing to Grafana

1. Generate the dashboard JSON file using the script
2. In Grafana, go to Dashboards → Import
3. Upload the JSON file or paste its contents
4. Select the Prometheus data source
5. Click Import

## Best Practices

1. **Reusability**: Use the panels library for common metrics
2. **Consistency**: Maintain consistent styling across dashboards
3. **Variables**: Use template variables for flexibility
4. **Documentation**: Document custom panels and their queries
5. **Versioning**: Track dashboard changes in version control

## Extending the Base Dashboard

To add new panels to the library:

1. Edit `base-dashboard.json`
2. Add your panel configuration to the `panels_library` object
3. Use meaningful keys that describe the panel's purpose
4. Include all necessary configuration for the panel to be self-contained

## Troubleshooting

### Common Issues

1. **Missing Data Source**: Ensure Prometheus is configured in Grafana
2. **No Data**: Check that metrics are being collected and scraped
3. **Query Errors**: Verify metric names match your Prometheus configuration
4. **Layout Issues**: Adjust grid positions in the generator script

### Debug Mode

Run the generator with Python's verbose flag for debugging:
```bash
python -v dashboard-generator.py anomaly-detector
```