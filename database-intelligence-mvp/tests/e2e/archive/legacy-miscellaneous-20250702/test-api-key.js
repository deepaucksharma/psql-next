#!/usr/bin/env node

// Simple test to verify New Relic API key
require('dotenv').config({ path: '../../.env' });
const https = require('https');

const NEW_RELIC_USER_KEY = process.env.NEW_RELIC_USER_KEY;
const NEW_RELIC_ACCOUNT_ID = process.env.NEW_RELIC_ACCOUNT_ID || '3630072';

console.log('Testing New Relic API Key...');
console.log(`Account ID: ${NEW_RELIC_ACCOUNT_ID}`);
console.log(`User Key exists: ${NEW_RELIC_USER_KEY ? 'Yes' : 'No'}`);

const query = `
query {
  actor {
    user {
      email
      name
    }
  }
}`;

const data = JSON.stringify({ query });

const options = {
  hostname: 'api.newrelic.com',
  path: '/graphql',
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'API-Key': NEW_RELIC_USER_KEY,
    'Content-Length': data.length
  }
};

const req = https.request(options, (res) => {
  let responseData = '';

  res.on('data', (chunk) => {
    responseData += chunk;
  });

  res.on('end', () => {
    try {
      const parsed = JSON.parse(responseData);
      if (parsed.errors) {
        console.error('❌ API Key validation failed:', parsed.errors);
      } else if (parsed.data && parsed.data.actor && parsed.data.actor.user) {
        console.log('✅ API Key is valid!');
        console.log(`   User: ${parsed.data.actor.user.name || parsed.data.actor.user.email}`);
      } else {
        console.log('❓ Unexpected response:', parsed);
      }
    } catch (e) {
      console.error('❌ Failed to parse response:', e);
      console.log('Response:', responseData);
    }
  });
});

req.on('error', (error) => {
  console.error('❌ Request failed:', error);
});

req.write(data);
req.end();