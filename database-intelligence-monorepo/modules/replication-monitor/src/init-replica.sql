-- Replica initialization script
-- This script sets up the replica server and starts replication

-- Wait for master to be ready
SELECT SLEEP(10);

-- Create monitoring user for collector
CREATE USER IF NOT EXISTS 'monitoring'@'%' IDENTIFIED BY 'monitoring_password';
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'monitoring'@'%';
GRANT SELECT ON performance_schema.* TO 'monitoring'@'%';
FLUSH PRIVILEGES;

-- Configure replication
STOP SLAVE;
RESET SLAVE ALL;

-- Set up replication using GTID
CHANGE MASTER TO
    MASTER_HOST='mysql-master',
    MASTER_USER='replication_user',
    MASTER_PASSWORD='replication_password',
    MASTER_PORT=3306,
    MASTER_AUTO_POSITION=1,
    MASTER_CONNECT_RETRY=10,
    MASTER_RETRY_COUNT=3,
    MASTER_HEARTBEAT_PERIOD=10;

-- Start replication
START SLAVE;

-- Wait for replication to catch up
SELECT SLEEP(5);

-- Show slave status
SHOW SLAVE STATUS\G

-- Create stored procedure to monitor replication lag
DELIMITER //

CREATE PROCEDURE check_replication_status()
BEGIN
    DECLARE io_running VARCHAR(10);
    DECLARE sql_running VARCHAR(10);
    DECLARE seconds_behind INT;
    DECLARE last_error TEXT;
    
    SELECT 
        Slave_IO_Running,
        Slave_SQL_Running,
        Seconds_Behind_Master,
        Last_Error
    INTO 
        io_running,
        sql_running,
        seconds_behind,
        last_error
    FROM information_schema.processlist
    WHERE command = 'Binlog Dump';
    
    SELECT 
        NOW() as check_time,
        io_running as io_thread_running,
        sql_running as sql_thread_running,
        seconds_behind as lag_seconds,
        last_error as last_replication_error;
END//

-- Create procedure to test replication lag under load
CREATE PROCEDURE test_replication_lag(IN duration_seconds INT)
BEGIN
    DECLARE start_time TIMESTAMP DEFAULT NOW();
    DECLARE end_time TIMESTAMP;
    DECLARE lag_sum INT DEFAULT 0;
    DECLARE lag_count INT DEFAULT 0;
    DECLARE current_lag INT;
    
    SET end_time = DATE_ADD(start_time, INTERVAL duration_seconds SECOND);
    
    WHILE NOW() < end_time DO
        -- Get current lag
        SELECT Seconds_Behind_Master INTO current_lag
        FROM information_schema.processlist
        WHERE command = 'Binlog Dump'
        LIMIT 1;
        
        IF current_lag IS NOT NULL THEN
            SET lag_sum = lag_sum + current_lag;
            SET lag_count = lag_count + 1;
        END IF;
        
        -- Wait 1 second between measurements
        DO SLEEP(1);
    END WHILE;
    
    -- Return average lag
    IF lag_count > 0 THEN
        SELECT 
            lag_count as measurements,
            lag_sum as total_lag,
            ROUND(lag_sum / lag_count, 2) as average_lag;
    ELSE
        SELECT 'No lag measurements available' as result;
    END IF;
END//

-- Create function to get GTID lag
CREATE FUNCTION get_gtid_lag() RETURNS INT
DETERMINISTIC
READS SQL DATA
BEGIN
    DECLARE master_gtid VARCHAR(1000);
    DECLARE slave_gtid VARCHAR(1000);
    DECLARE gtid_lag INT;
    
    -- This is a simplified example
    -- In production, you would parse and compare GTIDs properly
    SELECT @@gtid_executed INTO slave_gtid;
    
    -- For demonstration, return 0
    RETURN 0;
END//

DELIMITER ;

-- Create monitoring views
CREATE VIEW replication_status AS
SELECT 
    Slave_IO_State,
    Master_Host,
    Master_User,
    Master_Port,
    Connect_Retry,
    Master_Log_File,
    Read_Master_Log_Pos,
    Relay_Log_File,
    Relay_Log_Pos,
    Relay_Master_Log_File,
    Slave_IO_Running,
    Slave_SQL_Running,
    Last_Errno,
    Last_Error,
    Seconds_Behind_Master,
    Master_SSL_Allowed,
    Master_Server_Id,
    Master_UUID,
    SQL_Delay,
    SQL_Remaining_Delay,
    Master_Retry_Count,
    Auto_Position,
    Channel_Name
FROM information_schema.processlist
WHERE command = 'Binlog Dump';

CREATE VIEW replication_metrics AS
SELECT 
    NOW() as timestamp,
    (SELECT Seconds_Behind_Master FROM information_schema.processlist WHERE command = 'Binlog Dump' LIMIT 1) as replication_lag,
    (SELECT COUNT(*) FROM information_schema.processlist WHERE command = 'Binlog Dump') as slave_connections,
    @@server_id as server_id,
    @@hostname as hostname,
    'replica' as role;

-- Enable performance schema instruments for replication monitoring
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE '%replication%';

UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME LIKE '%replication%';

-- Create event to periodically check replication health
CREATE EVENT IF NOT EXISTS check_replication_health
ON SCHEDULE EVERY 1 MINUTE
DO
BEGIN
    DECLARE io_running VARCHAR(10);
    DECLARE sql_running VARCHAR(10);
    
    SELECT 
        Slave_IO_Running,
        Slave_SQL_Running
    INTO 
        io_running,
        sql_running
    FROM information_schema.processlist
    WHERE command = 'Binlog Dump'
    LIMIT 1;
    
    -- Log warning if replication is not running
    IF io_running != 'Yes' OR sql_running != 'Yes' THEN
        INSERT INTO mysql.general_log (event_time, user_host, thread_id, server_id, command_type, argument)
        VALUES (NOW(), 'replication_monitor@localhost', CONNECTION_ID(), @@server_id, 'Warning', 
                CONCAT('Replication issue detected - IO: ', IFNULL(io_running, 'NULL'), ', SQL: ', IFNULL(sql_running, 'NULL')));
    END IF;
END;

-- Enable event scheduler
SET GLOBAL event_scheduler = ON;