# pg_querylens Makefile

MODULE_big = pg_querylens
OBJS = pg_querylens.o

EXTENSION = pg_querylens
DATA = pg_querylens--1.0.sql
PGFILEDESC = "pg_querylens - zero-overhead query telemetry"

# PostgreSQL configuration
PG_CONFIG = pg_config
PGXS := $(shell $(PG_CONFIG) --pgxs)
include $(PGXS)

# Additional compiler flags for optimization
override CFLAGS += -O3 -march=native -mtune=native

# Installation targets
install: install-lib install-data

install-lib: all
	$(INSTALL_SHLIB) $(MODULE_big).so '$(DESTDIR)$(pkglibdir)/'

install-data:
	$(INSTALL_DATA) $(EXTENSION).control '$(DESTDIR)$(datadir)/extension/'
	$(INSTALL_DATA) $(DATA) '$(DESTDIR)$(datadir)/extension/'

# Development targets
dev-install: clean all install
	@echo "Development installation complete"

# Testing
check: all
	$(MAKE) -C test check

# Clean targets
clean:
	rm -f $(OBJS) $(MODULE_big).so

distclean: clean
	rm -f *~ tags

# Documentation
docs:
	@echo "Building documentation..."
	mkdir -p doc
	@echo "# pg_querylens" > doc/README.md
	@echo "" >> doc/README.md
	@echo "Zero-overhead query telemetry extension for PostgreSQL" >> doc/README.md
	@echo "" >> doc/README.md
	@echo "## Features" >> doc/README.md
	@echo "- Shared memory based collection (no overhead)" >> doc/README.md
	@echo "- Real-time event streaming" >> doc/README.md
	@echo "- Plan change detection" >> doc/README.md
	@echo "- Performance regression detection" >> doc/README.md
	@echo "- Configurable sampling" >> doc/README.md
	@echo "" >> doc/README.md
	@echo "## Installation" >> doc/README.md
	@echo "\`\`\`bash" >> doc/README.md
	@echo "make" >> doc/README.md
	@echo "sudo make install" >> doc/README.md
	@echo "\`\`\`" >> doc/README.md
	@echo "" >> doc/README.md
	@echo "## Usage" >> doc/README.md
	@echo "\`\`\`sql" >> doc/README.md
	@echo "CREATE EXTENSION pg_querylens;" >> doc/README.md
	@echo "" >> doc/README.md
	@echo "-- View query statistics" >> doc/README.md
	@echo "SELECT * FROM querylens.query_stats;" >> doc/README.md
	@echo "" >> doc/README.md
	@echo "-- View top queries" >> doc/README.md
	@echo "SELECT * FROM querylens.top_queries_by_time;" >> doc/README.md
	@echo "" >> doc/README.md
	@echo "-- Check for plan changes" >> doc/README.md
	@echo "SELECT * FROM querylens.plan_changes;" >> doc/README.md
	@echo "" >> doc/README.md
	@echo "-- Detect performance regressions" >> doc/README.md
	@echo "SELECT * FROM querylens.performance_regressions;" >> doc/README.md
	@echo "\`\`\`" >> doc/README.md

.PHONY: all install install-lib install-data clean distclean check dev-install docs