# Start Here! 🚀

## What is Database Intelligence MVP?

A complete solution for monitoring your PostgreSQL and MySQL databases, with two deployment options.

## Choose Your Path

### 🟢 Path 1: "I want it simple and stable"

**→ Use Standard Mode (Recommended)**

```bash
./quickstart.sh all
```

*   ✅ Production-ready today, no build required, low resource usage, high availability support.

### 🔵 Path 2: "I want advanced features"

**→ Use Experimental Mode**

```bash
./quickstart.sh --experimental all
```

*   🚀 Active Session History (1-second samples), smart adaptive sampling, automatic circuit breaker protection, multi-database federation.

## 30-Second Setup

```bash
# 1. Clone
git clone https://github.com/newrelic/database-intelligence-mvp
cd database-intelligence-mvp

# 2. Choose your mode and run
./quickstart.sh all                  # Standard
# OR
./quickstart.sh --experimental all   # Experimental

# 3. Follow the prompts (database connection, New Relic license key)
```

## What You Get

### With Standard Mode
*   Query performance metrics (5 min intervals), top slow queries, execution counts and timing, safe for all production databases.

### With Experimental Mode (All of the above plus)
*   Second-by-second session activity, smart sampling (100% slow, 25% fast), automatic protection, future-ready for query plan analysis.

## Quick Links

*   **First time?** → [GETTING-STARTED.md](GETTING-STARTED.md).
*   **Compare modes** → [DEPLOYMENT-OPTIONS.md](DEPLOYMENT-OPTIONS.md).
*   **Having issues?** → [TROUBLESHOOTING-GUIDE.md](TROUBLESHOOTING-GUIDE.md).
*   **Architecture** → [ARCHITECTURE.md](ARCHITECTURE.md).

## System Requirements

*   Docker & Docker Compose.
*   512MB RAM (Standard) or 2GB RAM (Experimental).
*   PostgreSQL 10+ or MySQL 5.7+.
*   New Relic account.

**Tip**: Start with Standard Mode unless you specifically need experimental features. You can always switch to Experimental later!
