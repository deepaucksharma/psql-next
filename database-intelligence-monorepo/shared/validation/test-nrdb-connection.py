#!/usr/bin/env python3
"""
Test New Relic Database Connection

This script tests the connection to New Relic and validates that the API credentials work.
It performs a simple NRQL query to verify connectivity and data access.

Usage:
    python3 test-nrdb-connection.py
"""

import os
import sys
import json
import requests
from datetime import datetime
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

def test_new_relic_connection():
    """Test connection to New Relic and basic NRQL functionality"""
    
    # Get credentials from environment
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
    session = requests.Session()
    session.headers.update({
        'Api-Key': api_key,
        'Content-Type': 'application/json'
    })
    
    # Simple test query to check connection
    test_query = f"""
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
    
    try:
        print("\n🔍 Testing New Relic GraphQL API connection...")
        response = session.post(
            "https://api.newrelic.com/graphql",
            json={"query": test_query},
            timeout=30
        )
        
        if response.status_code != 200:
            print(f"❌ API request failed with status {response.status_code}")
            print(f"Response: {response.text}")
            return False
        
        data = response.json()
        
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
        
    except requests.exceptions.RequestException as e:
        print(f"❌ Network error: {e}")
        return False
    except Exception as e:
        print(f"❌ Unexpected error: {e}")
        return False
    
    # Test module-specific data
    print("\n🔍 Checking for database intelligence module data...")
    
    modules_to_check = [
        'core-metrics',
        'sql-intelligence', 
        'wait-profiler',
        'anomaly-detector',
        'business-impact',
        'replication-monitor',
        'performance-advisor',
        'resource-monitor',
        'alert-manager',
        'canary-tester',
        'cross-signal-correlator'
    ]
    
    module_data_found = {}
    
    for module in modules_to_check:
        module_query = f"""
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
        
        try:
            response = session.post(
                "https://api.newrelic.com/graphql",
                json={"query": module_query},
                timeout=10
            )
            
            if response.status_code == 200:
                data = response.json()
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
            else:
                print(f"  ❌ {module}: API error")
                
        except Exception as e:
            print(f"  ❌ {module}: {e}")
    
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
    print("🚀 Testing New Relic Database Intelligence Connection")
    print("=" * 60)
    
    if test_new_relic_connection():
        print("\n✅ Connection test completed successfully")
        print("\nNext steps:")
        print("  1. Run full validation: python3 shared/validation/automated-nrdb-validator.py --validate-all")
        print("  2. Run module-specific validation: python3 shared/validation/module-specific/validate-core-metrics.py")
        print("  3. Check troubleshooting: ./shared/validation/troubleshoot-missing-data.sh --all")
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