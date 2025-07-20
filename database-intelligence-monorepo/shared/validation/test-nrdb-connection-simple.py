#!/usr/bin/env python3
"""
Simple New Relic Database Connection Test (No External Dependencies)

This script tests the connection to New Relic using only Python standard library,
providing a fallback when requests/dotenv are not available.

Usage:
    python3 test-nrdb-connection-simple.py
"""

import os
import sys
import json
import urllib.request
import urllib.parse
import urllib.error
from datetime import datetime

def load_env_variables():
    """Load environment variables from .env file manually"""
    env_file = '.env'
    if os.path.exists(env_file):
        with open(env_file, 'r') as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith('#') and '=' in line:
                    key, value = line.split('=', 1)
                    os.environ[key.strip()] = value.strip()

def test_new_relic_connection():
    """Test connection to New Relic using urllib"""
    
    # Load environment variables
    load_env_variables()
    
    # Get credentials
    api_key = os.getenv('NEW_RELIC_API_KEY')
    account_id = os.getenv('NEW_RELIC_ACCOUNT_ID')
    
    if not api_key:
        print("❌ NEW_RELIC_API_KEY not found in environment")
        return False
    if not account_id:
        print("❌ NEW_RELIC_ACCOUNT_ID not found in environment")
        return False
    
    print(f"✓ Found API credentials")
    print(f"  Account ID: {account_id}")
    print(f"  API Key: {api_key[:8]}...")
    
    # Test GraphQL API connection
    test_query = {
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
        print("\n🔍 Testing New Relic GraphQL API connection...")
        
        # Prepare request
        url = "https://api.newrelic.com/graphql"
        data = json.dumps(test_query).encode('utf-8')
        
        req = urllib.request.Request(url, data=data)
        req.add_header('Api-Key', api_key)
        req.add_header('Content-Type', 'application/json')
        
        # Make request
        with urllib.request.urlopen(req, timeout=30) as response:
            data = json.loads(response.read().decode('utf-8'))
            
            if 'errors' in data:
                print(f"❌ GraphQL errors: {data['errors']}")
                return False
            
            results = data.get('data', {}).get('actor', {}).get('account', {}).get('nrql', {}).get('results', [])
            
            if not results:
                print("⚠️  API connection successful but no metric data found")
                print("   This might be normal if no data has been sent to New Relic yet")
            else:
                total_metrics = results[0].get('count', 0)
                print(f"✅ API connection successful!")
                print(f"   Found {total_metrics} metric records in the last hour")
        
    except urllib.error.HTTPError as e:
        print(f"❌ HTTP Error {e.code}: {e.reason}")
        if e.code == 401:
            print("   Check your NEW_RELIC_API_KEY")
        elif e.code == 403:
            print("   API key may not have required permissions")
        return False
    except urllib.error.URLError as e:
        print(f"❌ Network error: {e.reason}")
        return False
    except Exception as e:
        print(f"❌ Unexpected error: {e}")
        return False
    
    # Test module-specific data
    print("\n🔍 Checking for database intelligence module data...")
    
    modules_to_check = [
        'core-metrics', 'sql-intelligence', 'wait-profiler', 'anomaly-detector',
        'business-impact', 'replication-monitor', 'performance-advisor', 
        'resource-monitor', 'alert-manager', 'canary-tester', 'cross-signal-correlator'
    ]
    
    module_data_found = {}
    
    for module in modules_to_check:
        module_query = {
            "query": f"""
            {{
                actor {{
                    account(id: {account_id}) {{
                        nrql(query: "SELECT count(*) FROM Metric WHERE service.name = '{module}' SINCE 1 hour ago LIMIT 1") {{
                            results
                        }}
                    }}
                }}
            }}
            """
        }
        
        try:
            data = json.dumps(module_query).encode('utf-8')
            req = urllib.request.Request("https://api.newrelic.com/graphql", data=data)
            req.add_header('Api-Key', api_key)
            req.add_header('Content-Type', 'application/json')
            
            with urllib.request.urlopen(req, timeout=10) as response:
                data = json.loads(response.read().decode('utf-8'))
                
                if 'errors' not in data:
                    results = data.get('data', {}).get('actor', {}).get('account', {}).get('nrql', {}).get('results', [])
                    if results:
                        count = results[0].get('count', 0)
                        module_data_found[module] = count
                        if count > 0:
                            print(f"  ✅ {module}: {count} metric records")
                        else:
                            print(f"  ⚪ {module}: no data")
                    else:
                        print(f"  ⚪ {module}: no data")
                else:
                    print(f"  ❌ {module}: query error")
                
        except Exception:
            print(f"  ❌ {module}: connection error")
    
    # Summary
    total_modules = len(modules_to_check)
    modules_with_data = len([m for m, count in module_data_found.items() if count > 0])
    
    print(f"\n📊 Summary:")
    print(f"   Modules with data: {modules_with_data}/{total_modules}")
    
    if modules_with_data == 0:
        print("   ⚠️  No module data found. This is expected if:")
        print("      - Modules are not yet deployed")
        print("      - Data collection has not started")
        print("      - Modules are sending data but not in the last hour")
    elif modules_with_data < total_modules:
        print("   ⚠️  Some modules missing data. Check module health and configuration.")
    else:
        print("   ✅ All modules are sending data!")
    
    return True

def main():
    print("🚀 Testing New Relic Database Intelligence Connection (Simple Version)")
    print("=" * 70)
    
    if test_new_relic_connection():
        print("\n✅ Connection test completed successfully")
        print("\nNext steps:")
        print("  1. Run full validation: python3 shared/validation/run-comprehensive-validation.py --quick")
        print("  2. Check individual modules: python3 shared/validation/module-specific/validate-core-metrics.py")
        print("  3. Run troubleshooting: ./shared/validation/troubleshoot-missing-data.sh --all")
        sys.exit(0)
    else:
        print("\n❌ Connection test failed")
        print("\nTroubleshooting:")
        print("  1. Check .env file contains correct NEW_RELIC_API_KEY and NEW_RELIC_ACCOUNT_ID")
        print("  2. Verify API key has NerdGraph access permissions")
        print("  3. Check network connectivity to api.newrelic.com")
        sys.exit(1)

if __name__ == '__main__':
    main()