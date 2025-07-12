# CI/CD Workflows

This directory contains all continuous integration and deployment workflows.

## Available Workflows

### 1. Continuous Integration (`ci.yml`)
Runs on every push and pull request:
- Code linting and formatting
- Unit tests
- Integration tests
- Build verification
- Security scanning

### 2. Enhanced CI (`ci-enhanced.yml`)
Extended CI for main branch:
- All standard CI checks
- E2E tests
- Performance benchmarks
- Custom component tests
- Multi-platform builds

### 3. End-to-End Tests (`e2e-tests.yml`)
Comprehensive E2E testing:
- Spin up test databases
- Deploy collector configurations
- Verify metric collection
- Validate New Relic integration
- Load testing

### 4. Continuous Deployment (`cd.yml`)
Automated deployment pipeline:
- Build Docker images
- Push to registry
- Deploy to staging
- Run smoke tests
- Deploy to production (manual approval)

### 5. Release (`release.yml`)
Release automation:
- Version tagging
- Changelog generation
- Binary builds for all platforms
- Docker image publishing
- GitHub release creation

## Workflow Triggers

| Workflow | Trigger | Branches |
|----------|---------|----------|
| CI | Push, PR | All |
| Enhanced CI | Push | main, release/* |
| E2E Tests | Schedule (daily), Manual | main |
| CD | Push | main (after CI) |
| Release | Tag | v* tags |

## Environment Setup

### Required Secrets
```yaml
# GitHub Actions secrets required:
NEW_RELIC_LICENSE_KEY    # New Relic license
NEW_RELIC_ACCOUNT_ID     # New Relic account
DOCKER_REGISTRY_USERNAME # Docker Hub username
DOCKER_REGISTRY_PASSWORD # Docker Hub password
GHCR_TOKEN              # GitHub Container Registry token
```

### Environment Variables
```yaml
# Set in workflow files:
GO_VERSION: "1.22"
OTEL_VERSION: "0.105.0"
NODE_VERSION: "18"
```

## Local Testing

Test workflows locally using [act](https://github.com/nektos/act):

```bash
# Test CI workflow
act -W ci-cd/ci.yml

# Test with specific event
act pull_request -W ci-cd/ci.yml

# Test with secrets
act -W ci-cd/cd.yml --secret-file .secrets
```

## Workflow Details

### CI Pipeline Stages

1. **Setup**
   - Checkout code
   - Setup Go environment
   - Cache dependencies

2. **Validation**
   - Go mod tidy check
   - Code formatting (gofmt)
   - Linting (golangci-lint)

3. **Testing**
   - Unit tests with coverage
   - Integration tests
   - Race condition detection

4. **Build**
   - Build all distributions
   - Verify component registration

5. **Security**
   - Vulnerability scanning (govulncheck)
   - License compliance
   - SAST scanning

### CD Pipeline Stages

1. **Build & Package**
   - Multi-arch Docker builds
   - Binary compilation
   - Configuration validation

2. **Staging Deployment**
   - Deploy to staging environment
   - Run smoke tests
   - Performance validation

3. **Production Deployment**
   - Manual approval required
   - Blue-green deployment
   - Automated rollback on failure

## Best Practices

1. **PR Checks**: All PRs must pass CI before merge
2. **Branch Protection**: Main branch requires PR reviews
3. **Semantic Versioning**: Use conventional commits
4. **Secret Management**: Never commit secrets
5. **Cache Optimization**: Use GitHub Actions cache

## Troubleshooting

### Common Issues

1. **Build Failures**
   - Check Go version compatibility
   - Verify module dependencies
   - Review builder configurations

2. **Test Failures**
   - Check test database connectivity
   - Verify environment variables
   - Review test logs in artifacts

3. **Deployment Failures**
   - Verify registry credentials
   - Check deployment permissions
   - Review deployment logs

### Debug Mode

Enable debug logging in workflows:
```yaml
env:
  ACTIONS_RUNNER_DEBUG: true
  ACTIONS_STEP_DEBUG: true
```

## Maintenance

### Updating Dependencies
- Dependabot configured for automatic updates
- Review and test dependency updates
- Update Go version in all workflows

### Workflow Optimization
- Monitor workflow run times
- Optimize cache usage
- Parallelize where possible
- Use matrix builds efficiently