// MongoDB initialization script for E2E testing
// This script sets up users, databases, and initial test data

// Switch to admin database
db = db.getSiblingDB('admin');

// Create monitoring user with necessary permissions
db.createUser({
  user: 'monitoring',
  pwd: 'monitoring_password',
  roles: [
    { role: 'clusterMonitor', db: 'admin' },
    { role: 'read', db: 'local' },
    { role: 'readAnyDatabase', db: 'admin' },
    { role: 'listDatabases', db: 'admin' },
    { role: 'listCollections', db: 'admin' },
    { role: 'dbStats', db: 'admin' },
    { role: 'collStats', db: 'admin' }
  ]
});

// Create application user
db.createUser({
  user: 'appuser',
  pwd: 'apppassword',
  roles: [
    { role: 'readWrite', db: 'testdb' },
    { role: 'readWrite', db: 'e2e_test' }
  ]
});

// Switch to test database
db = db.getSiblingDB('testdb');

// Create collections with different characteristics
db.createCollection('users', {
  validator: {
    $jsonSchema: {
      bsonType: 'object',
      required: ['email', 'name'],
      properties: {
        email: {
          bsonType: 'string',
          pattern: '^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$'
        },
        name: {
          bsonType: 'string',
          minLength: 1
        },
        age: {
          bsonType: 'int',
          minimum: 0,
          maximum: 150
        }
      }
    }
  }
});

db.createCollection('products', {
  capped: false
});

db.createCollection('logs', {
  capped: true,
  size: 10485760, // 10MB
  max: 10000
});

// Create time series collection (MongoDB 5.0+)
try {
  db.createCollection('metrics', {
    timeseries: {
      timeField: 'timestamp',
      metaField: 'metadata',
      granularity: 'seconds'
    }
  });
} catch (e) {
  print('Time series collection not supported in this MongoDB version');
}

// Create indexes
db.users.createIndex({ email: 1 }, { unique: true });
db.users.createIndex({ name: 1 });
db.users.createIndex({ age: -1 });
db.users.createIndex({ 'metadata.source': 1, created_at: -1 });

db.products.createIndex({ sku: 1 }, { unique: true });
db.products.createIndex({ category: 1, price: -1 });
db.products.createIndex({ name: 'text' });

// Insert sample users
var sampleUsers = [];
for (var i = 0; i < 1000; i++) {
  sampleUsers.push({
    email: 'user' + i + '@example.com',
    name: 'Test User ' + i,
    age: Math.floor(Math.random() * 80) + 18,
    created_at: new Date(Date.now() - Math.random() * 365 * 24 * 60 * 60 * 1000),
    last_login: new Date(Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000),
    active: Math.random() > 0.2,
    metadata: {
      source: ['web', 'mobile', 'api'][Math.floor(Math.random() * 3)],
      ip: '192.168.' + Math.floor(Math.random() * 255) + '.' + Math.floor(Math.random() * 255),
      user_agent: 'Mozilla/5.0'
    },
    preferences: {
      newsletter: Math.random() > 0.5,
      notifications: Math.random() > 0.3,
      theme: ['light', 'dark', 'auto'][Math.floor(Math.random() * 3)]
    },
    tags: ['user', 'test', 'sample']
  });
}
db.users.insertMany(sampleUsers);

// Insert sample products
var categories = ['Electronics', 'Clothing', 'Books', 'Food', 'Toys'];
var sampleProducts = [];
for (var i = 0; i < 500; i++) {
  sampleProducts.push({
    sku: 'SKU-' + (1000 + i),
    name: 'Product ' + i,
    description: 'This is a test product with a detailed description for testing purposes.',
    category: categories[Math.floor(Math.random() * categories.length)],
    price: Math.round(Math.random() * 1000 * 100) / 100,
    cost: Math.round(Math.random() * 500 * 100) / 100,
    stock: Math.floor(Math.random() * 1000),
    warehouse_location: 'W' + Math.floor(Math.random() * 10),
    created_at: new Date(),
    updated_at: new Date(),
    attributes: {
      color: ['Red', 'Blue', 'Green', 'Black', 'White'][Math.floor(Math.random() * 5)],
      size: ['S', 'M', 'L', 'XL'][Math.floor(Math.random() * 4)],
      weight: Math.random() * 10
    }
  });
}
db.products.insertMany(sampleProducts);

// Insert sample logs
for (var i = 0; i < 100; i++) {
  db.logs.insert({
    timestamp: new Date(),
    level: ['DEBUG', 'INFO', 'WARN', 'ERROR'][Math.floor(Math.random() * 4)],
    service: ['auth', 'api', 'db', 'cache'][Math.floor(Math.random() * 4)],
    message: 'Sample log message ' + i,
    details: {
      request_id: 'req-' + Math.random().toString(36).substr(2, 9),
      user_id: Math.floor(Math.random() * 1000),
      duration_ms: Math.floor(Math.random() * 1000)
    }
  });
}

// Create views
db.createView('active_users', 'users', [
  { $match: { active: true } },
  { $project: { email: 1, name: 1, last_login: 1 } }
]);

db.createView('product_inventory', 'products', [
  { $match: { stock: { $gt: 0 } } },
  { $group: {
    _id: '$category',
    total_products: { $sum: 1 },
    total_stock: { $sum: '$stock' },
    avg_price: { $avg: '$price' }
  }}
]);

// Create stored functions for testing
db.system.js.save({
  _id: 'calculateRevenue',
  value: function(productId, quantity) {
    var product = db.products.findOne({ _id: productId });
    if (product) {
      return product.price * quantity;
    }
    return 0;
  }
});

// Enable profiling for slow query detection
db.setProfilingLevel(1, { slowms: 100 });

// Create E2E test database
db = db.getSiblingDB('e2e_test');

// Create test collection with PII data for sanitization testing
db.test_pii.insertMany([
  {
    name: 'John Doe',
    email: 'john.doe@example.com',
    ssn: '123-45-6789',
    credit_card: '4111-1111-1111-1111',
    phone: '+1-555-123-4567',
    address: '123 Main St, Anytown, USA'
  },
  {
    name: 'Jane Smith',
    email: 'jane.smith@example.com',
    ssn: '987-65-4321',
    credit_card: '5500-0000-0000-0004',
    phone: '+1-555-987-6543',
    address: '456 Oak Ave, Somewhere, USA'
  }
]);

// Create collection for query pattern testing
db.query_patterns.insertMany([
  { type: 'simple', value: 1 },
  { type: 'simple', value: 2 },
  { type: 'complex', nested: { field1: 'value1', field2: 'value2' } },
  { type: 'array', items: [1, 2, 3, 4, 5] }
]);

// Create collection for performance testing
var perfData = [];
for (var i = 0; i < 10000; i++) {
  perfData.push({
    id: i,
    timestamp: new Date(Date.now() - i * 1000),
    value: Math.sin(i / 100) * 100,
    category: 'cat' + (i % 10),
    tags: ['tag' + (i % 5), 'tag' + (i % 7)]
  });
}
db.performance_test.insertMany(perfData);
db.performance_test.createIndex({ timestamp: -1 });
db.performance_test.createIndex({ category: 1, value: -1 });

// Output initialization summary
print('MongoDB E2E test initialization complete');
print('Created databases: admin, testdb, e2e_test');
print('Created users: monitoring, appuser');
print('Inserted sample data:');
print('  - users: ' + db.getSiblingDB('testdb').users.count() + ' documents');
print('  - products: ' + db.getSiblingDB('testdb').products.count() + ' documents');
print('  - logs: ' + db.getSiblingDB('testdb').logs.count() + ' documents');
print('  - performance_test: ' + db.performance_test.count() + ' documents');