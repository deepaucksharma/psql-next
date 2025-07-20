#!/usr/bin/env python3
"""
Test Validation Setup

This script tests the validation framework setup without external dependencies,
using only Python standard library modules to verify the validation environment.

Usage:
    python3 test-validation-setup.py
"""

import os
import sys
import json
import urllib.request
import urllib.parse
import urllib.error
from datetime import datetime
import logging

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

def load_env_file():
    """Load environment variables from .env file"""
    env_vars = {}
    env_file = '.env'
    
    if os.path.exists(env_file):
        with open(env_file, 'r') as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith('#') and '=' in line:
                    key, value = line.split('=', 1)
                    env_vars[key.strip()] = value.strip()
                    os.environ[key.strip()] = value.strip()
        print(f"✓ Loaded {len(env_vars)} environment variables from .env")
    else:
        print("❌ .env file not found")
        return False
    
    return True

def test_environment_variables():
    """Test that required environment variables are set"""
    required_vars = [
        'NEW_RELIC_API_KEY',
        'NEW_RELIC_ACCOUNT_ID',
        'NEW_RELIC_OTLP_ENDPOINT',
        'NEW_RELIC_LICENSE_KEY'
    ]
    
    print("\n🔍 Testing environment variables...")
    
    all_present = True
    for var in required_vars:
        value = os.getenv(var)
        if value:
            # Show partial value for security
            masked_value = value[:8] + '...' if len(value) > 8 else value
            print(f"  ✓ {var}: {masked_value}")
        else:
            print(f"  ❌ {var}: Not set")
            all_present = False
    
    return all_present

def test_new_relic_api_basic():
    """Test basic New Relic API connectivity using urllib"""
    api_key = os.getenv('NEW_RELIC_API_KEY')
    account_id = os.getenv('NEW_RELIC_ACCOUNT_ID')
    
    if not api_key or not account_id:
        print("❌ Cannot test API - missing credentials")
        return False
    
    print("\n🔗 Testing New Relic API connectivity...")
    
    # Simple GraphQL query
    query = {
        "query": f"""
        {{
            actor {{
                account(id: {account_id}) {{
                    nrql(query: "SELECT count(*) FROM Metric SINCE 1 hour ago LIMIT 1") {{
                        results
                    }}
                }}
            }}
        }}
        """
    }
    
    try:
        # Prepare request
        url = "https://api.newrelic.com/graphql"
        data = json.dumps(query).encode('utf-8')
        
        req = urllib.request.Request(url, data=data)
        req.add_header('Api-Key', api_key)
        req.add_header('Content-Type', 'application/json')
        
        # Make request
        with urllib.request.urlopen(req, timeout=30) as response:
            response_data = json.loads(response.read().decode('utf-8'))
            
            if 'errors' in response_data:
                print(f"❌ API returned errors: {response_data['errors']}")
                return False
            
            results = response_data.get('data', {}).get('actor', {}).get('account', {}).get('nrql', {}).get('results', [])
            
            if results:
                count = results[0].get('count', 0)
                print(f"✅ API connection successful! Found {count} metric records in last hour")
            else:
                print("⚠️  API connection successful but no data found (this is normal if no modules are running)")
            
            return True
            
    except urllib.error.HTTPError as e:
        print(f"❌ HTTP Error {e.code}: {e.reason}")
        if e.code == 401:
            print("   Check your NEW_RELIC_API_KEY")
        elif e.code == 403:
            print("   API key may not have required permissions")
        return False
    except urllib.error.URLError as e:
        print(f"❌ Network Error: {e.reason}")
        return False
    except Exception as e:
        print(f"❌ Unexpected error: {e}")
        return False

def test_validation_scripts():
    """Test that validation scripts exist and have correct permissions"""
    print("\n📁 Testing validation script files...")
    
    validation_scripts = [
        'shared/validation/test-nrdb-connection.py',
        'shared/validation/automated-nrdb-validator.py', 
        'shared/validation/end-to-end-pipeline-validator.py',
        'shared/validation/integration-test-suite.py',
        'shared/validation/run-comprehensive-validation.py',
        'shared/validation/troubleshoot-missing-data.sh',
        'shared/validation/nrdb-validation-queries.nrql',
        'shared/validation/module-specific/validate-core-metrics.py',
        'shared/validation/module-specific/validate-sql-intelligence.py',
        'shared/validation/module-specific/validate-anomaly-detector.py',
        'shared/validation/module-specific/validate-all-modules.py'
    ]
    
    all_exist = True
    for script in validation_scripts:
        if os.path.exists(script):
            # Check if executable
            is_executable = os.access(script, os.X_OK)
            executable_status = "✓ executable" if is_executable else "⚠ not executable"
            print(f"  ✓ {script} ({executable_status})")
        else:
            print(f"  ❌ {script} - missing")
            all_exist = False
    
    return all_exist

def test_python_imports():
    """Test that all required Python modules can be imported"""
    print("\n🐍 Testing Python module imports...")
    
    # Test standard library imports
    stdlib_modules = [
        'json', 'os', 'sys', 'time', 'datetime', 'logging', 
        'subprocess', 'argparse', 'urllib.request', 'urllib.parse',
        'dataclasses', 'enum', 'concurrent.futures', 'statistics'
    ]
    
    all_imports_ok = True
    for module in stdlib_modules:
        try:
            __import__(module)
            print(f"  ✓ {module}")
        except ImportError as e:
            print(f"  ❌ {module} - {e}")
            all_imports_ok = False
    
    # Test optional imports (that our scripts try to use)
    optional_modules = [
        ('requests', 'HTTP client library'),
        ('dotenv', 'Environment variable loader'),
        ('yaml', 'YAML parser')
    ]
    
    print("\n  Optional modules (will use fallbacks if missing):")
    for module, description in optional_modules:
        try:
            __import__(module)
            print(f"  ✓ {module} - {description}")
        except ImportError:
            print(f"  ⚠ {module} - {description} (missing, will use fallback)")
    
    return all_imports_ok

def test_file_permissions():
    """Test file permissions and directory structure"""
    print("\n🔒 Testing file permissions...")
    
    # Test if we can write to validation directory
    test_file = 'shared/validation/test_write.tmp'
    try:
        with open(test_file, 'w') as f:
            f.write('test')
        os.remove(test_file)
        print("  ✓ Can write to validation directory")
    except Exception as e:
        print(f"  ❌ Cannot write to validation directory: {e}")
        return False
    
    # Test if we can execute scripts
    test_script = 'shared/validation/test-nrdb-connection.py'
    if os.path.exists(test_script):
        if os.access(test_script, os.X_OK):
            print("  ✓ Validation scripts are executable")
        else:
            print("  ⚠ Validation scripts not executable (may need chmod +x)")
    
    return True

def generate_setup_report():
    """Generate a comprehensive setup report"""
    print("\n" + "="*60)
    print("VALIDATION FRAMEWORK SETUP REPORT")
    print("="*60)
    
    # Run all tests
    tests = [
        ("Environment File", load_env_file),
        ("Environment Variables", test_environment_variables),
        ("New Relic API", test_new_relic_api_basic),
        ("Validation Scripts", test_validation_scripts),
        ("Python Imports", test_python_imports),
        ("File Permissions", test_file_permissions)
    ]
    
    results = []
    for test_name, test_func in tests:
        try:
            result = test_func()
            results.append((test_name, result))
        except Exception as e:
            print(f"❌ {test_name} failed with exception: {e}")
            results.append((test_name, False))
    
    # Summary
    print(f"\n📊 Test Results Summary:")
    passed = 0
    total = len(results)
    
    for test_name, result in results:
        status = "✅ PASS" if result else "❌ FAIL"
        print(f"  {status} {test_name}")
        if result:
            passed += 1
    
    success_rate = (passed / total) * 100 if total > 0 else 0
    print(f"\nOverall: {passed}/{total} tests passed ({success_rate:.1f}%)")
    
    # Recommendations
    print(f"\n💡 Next Steps:")
    if success_rate == 100:
        print("  🎉 All tests passed! Validation framework is ready to use.")
        print("  ▶️  Run: python3 shared/validation/run-comprehensive-validation.py --quick")
    elif success_rate >= 70:
        print("  ⚠️  Most tests passed. Address any failures above.")
        print("  ▶️  You can try: python3 shared/validation/test-nrdb-connection.py")
    else:
        print("  🚨 Multiple setup issues detected. Please address:")
        print("     1. Ensure .env file has correct New Relic credentials")
        print("     2. Check network connectivity to api.newrelic.com")
        print("     3. Verify file permissions")
    
    return success_rate >= 70

def main():
    print("🚀 Testing Database Intelligence Validation Framework Setup")
    print("=" * 60)
    
    success = generate_setup_report()
    
    if success:
        print("\n✅ Setup validation completed successfully!")
        sys.exit(0)
    else:
        print("\n❌ Setup validation found issues that need to be addressed.")
        sys.exit(1)

if __name__ == '__main__':
    main()