#!/bin/bash
# Test PII queries to validate sanitization

# PostgreSQL PII queries
echo "Testing PostgreSQL PII queries..."
docker exec e2e-postgres psql -U postgres -d e2e_test << EOF
-- Query with email
SELECT * FROM e2e_test.users WHERE email = 'john.doe@example.com';

-- Query with SSN
SELECT * FROM e2e_test.users WHERE ssn = '123-45-6789';

-- Query with credit card
SELECT * FROM e2e_test.users WHERE credit_card = '4111-1111-1111-1111';

-- Query with phone
SELECT * FROM e2e_test.users WHERE phone = '555-123-4567';

-- Insert with PII
INSERT INTO e2e_test.users (email, ssn, phone, credit_card, name) 
VALUES ('test.user@example.com', '999-88-7777', '555-999-8888', '5500-0000-0000-0004', 'Test User')
ON CONFLICT (email) DO NOTHING;

-- Complex query with multiple PII fields
SELECT u.email, u.ssn, u.credit_card, COUNT(o.id) as order_count
FROM e2e_test.users u
LEFT JOIN e2e_test.orders o ON u.id = o.user_id
WHERE u.email LIKE '%@example.com'
GROUP BY u.email, u.ssn, u.credit_card;
EOF

# MySQL PII queries
echo -e "\nTesting MySQL PII queries..."
docker exec e2e-mysql mysql -uroot -proot e2e_test << EOF
-- Query with email
SELECT * FROM users WHERE email = 'jane.smith@example.com';

-- Query with SSN
SELECT * FROM users WHERE ssn = '987-65-4321';

-- Query with credit card
SELECT * FROM users WHERE credit_card = '5555-5555-5555-4444';

-- Insert with PII
INSERT INTO users (email, ssn, phone, credit_card, name) 
VALUES ('mysql.test@example.com', '111-22-3333', '555-111-2222', '4000-0000-0000-0002', 'MySQL Test User')
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- Call stored procedure that contains PII
CALL generate_expensive_query();
EOF

echo -e "\nPII queries completed. Check collector output for sanitization."