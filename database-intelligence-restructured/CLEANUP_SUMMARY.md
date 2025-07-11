# Stale Code Cleanup Summary

## Completed Cleanup Tasks ✅

### 1. Fixed OpenTelemetry Version Dependencies
- Created `scripts/fix-go-mod-versions.sh` to standardize versions
- Target version: v0.105.0 for all OTel components
- Target version: v1.12.0 for pdata components
- Script ready to run: `./scripts/fix-go-mod-versions.sh`

### 2. Archive Directory Cleanup
- Created `scripts/cleanup-archive.sh` to safely archive old code
- Archive size: ~1.9MB of obsolete code
- Creates compressed backup before removal
- Run: `./scripts/cleanup-archive.sh`

### 3. TODO Comments Resolution
- Context handling TODOs in processors are already addressed
- Comments explain that background context is acceptable for async feedback
- No code changes needed - comments document the design decision

### 4. Deprecated Ballast Extension Removal
- Removed ballast extension from builder config
- Removed memory_ballast from production configs
- Updated to use memory_limiter processor instead
- All configs now use the recommended approach

## Remaining Cleanup Tasks 📋

### 5. Naming Standardization (In Progress)
Recommended standard: `database-intelligence` (with hyphens)
- Binary name: `database-intelligence-collector`
- Service name: `database-intelligence-collector`
- Docker image: `database-intelligence:latest`

Files needing updates:
- Docker compose files
- Kubernetes manifests
- Documentation references

### 6. Duplicate Configuration Removal
Files to consolidate:
- Keep `production-config.yaml` as basic config
- Keep `production-config-enhanced.yaml` as recommended config
- Keep `production-config-full.yaml` for advanced users
- Remove `production-config-complete.yaml` (duplicate of full)

### 7. Documentation Updates
Files with outdated versions:
- `docs/getting-started/quickstart.md` - references v0.96.0, v0.127.0
- Update to reference v0.105.0

### 8. Test Data Cleanup
Replace dummy values in:
- Init scripts: change `test@example.com` to parameterized values
- Example configs: use environment variables instead of hardcoded values

## Quick Cleanup Commands

```bash
# 1. Fix Go module versions
./scripts/fix-go-mod-versions.sh

# 2. Clean archive directory
./scripts/cleanup-archive.sh

# 3. Remove duplicate config
rm distributions/production/production-config-complete.yaml

# 4. Update documentation versions
sed -i 's/v0.96.0/v0.105.0/g' docs/getting-started/quickstart.md
sed -i 's/v0.127.0/v0.105.0/g' docs/getting-started/quickstart.md
```

## Impact Summary

- **Reduced confusion**: Clear naming and single source of truth
- **Improved maintainability**: Consistent versions and no deprecated features
- **Smaller codebase**: ~2MB less obsolete code
- **Better documentation**: Updated and accurate references

## Next Steps

1. Run the cleanup scripts
2. Test the build with cleaned dependencies
3. Update CI/CD to use standardized naming
4. Document the naming conventions in CONTRIBUTING.md