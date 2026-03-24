# Branch Protection Configuration

## Recommended Settings for `main` Branch

To ensure code quality and prevent broken code from being merged, configure the following branch protection rules for the `main` branch.

### How to Configure

1. Go to your repository on GitHub
2. Navigate to **Settings** → **Branches**
3. Click **Add rule** or edit existing rule for `main`
4. Apply the settings below

## Required Status Checks

Enable "Require status checks to pass before merging" and select:

- ✅ `Test (1.22)` - Tests on Go 1.22
- ✅ `Test (1.23)` - Tests on Go 1.23
- ✅ `Lint` - Code linting
- ✅ `Build (1.22)` - Build on Go 1.22
- ✅ `Build (1.23)` - Build on Go 1.23
- ✅ `Security Scan` - Security analysis

### Optional Status Checks

- ⚠️ `codecov/patch` - Coverage for changed code (if Codecov is configured)
- ⚠️ `codecov/project` - Overall coverage (if Codecov is configured)

## Recommended Protection Rules

### Basic Protection

```
☑ Require a pull request before merging
  ☑ Require approvals: 1
  ☐ Dismiss stale pull request approvals when new commits are pushed
  ☐ Require review from Code Owners

☑ Require status checks to pass before merging
  ☑ Require branches to be up to date before merging
  
☑ Require conversation resolution before merging

☐ Require signed commits (optional, for enhanced security)

☐ Require linear history (optional, for cleaner git history)

☑ Include administrators (recommended for consistency)
```

### Advanced Protection (Optional)

```
☐ Require deployments to succeed before merging
  (Enable if you have deployment workflows)

☐ Lock branch
  (Only for production branches that should never be directly modified)

☑ Do not allow bypassing the above settings
  (Ensures rules apply to everyone)

☑ Allow force pushes: Nobody
  (Prevents history rewriting)

☑ Allow deletions: Nobody
  (Prevents accidental branch deletion)
```

## Status Check Details

### Test Jobs

**Purpose**: Ensure all tests pass on supported Go versions

**What it checks**:
- Unit tests execute successfully
- No race conditions detected
- Code coverage is generated

**Failure reasons**:
- Test failures
- Race conditions
- Compilation errors

### Lint Job

**Purpose**: Enforce code quality standards

**What it checks**:
- Code formatting (gofmt)
- Import organization (goimports)
- Common mistakes (govet)
- Security issues (gosec)
- Code smells (gocritic, revive)
- Unused code (unused, ineffassign)

**Failure reasons**:
- Formatting issues
- Linting violations
- Security concerns
- Code quality issues

### Build Jobs

**Purpose**: Verify code compiles on all supported Go versions

**What it checks**:
- All packages build successfully
- Main binary can be created
- No compilation errors

**Failure reasons**:
- Syntax errors
- Import errors
- Type errors
- Missing dependencies

### Security Job

**Purpose**: Identify security vulnerabilities

**What it checks**:
- SQL injection risks
- Command injection risks
- Path traversal issues
- Weak cryptography
- Hardcoded credentials
- Other security anti-patterns

**Note**: This job runs in no-fail mode, so it won't block PRs but will report issues.

## Bypass Procedures

### Emergency Hotfixes

If you need to bypass protection rules in an emergency:

1. **Preferred**: Create a hotfix branch and follow normal PR process with expedited review
2. **Last Resort**: Temporarily disable protection, merge, then re-enable immediately

### Administrator Override

Administrators can bypass protection rules if "Include administrators" is unchecked, but this is **not recommended** for maintaining code quality.

## Monitoring

### Check Status

View status check results:
- On pull request page: See all checks at the bottom
- In Actions tab: View detailed logs for each job
- In Security tab: View Gosec findings

### Failed Checks

When a check fails:
1. Click "Details" next to the failed check
2. Review the error logs
3. Fix the issue locally
4. Push the fix to the PR branch
5. Checks will re-run automatically

## Maintenance

### Updating Required Checks

When you add new jobs to CI workflow:
1. Update branch protection rules
2. Add new job names to required status checks
3. Test with a draft PR first

### Removing Deprecated Checks

When removing jobs from CI:
1. Remove from required status checks first
2. Wait for all open PRs to merge
3. Then remove from workflow file

## Best Practices

### For Contributors

- ✅ Run `make ci` locally before pushing
- ✅ Keep PRs small and focused
- ✅ Write tests for new features
- ✅ Update documentation as needed
- ✅ Respond to review comments promptly

### For Maintainers

- ✅ Review PRs within 24-48 hours
- ✅ Provide constructive feedback
- ✅ Ensure CI passes before merging
- ✅ Keep branch protection rules up to date
- ✅ Monitor security findings regularly

## Troubleshooting

### "Required status check is missing"

**Cause**: Job name in workflow doesn't match required check name

**Solution**: 
1. Check exact job name in `.github/workflows/ci.yml`
2. Update branch protection to match exact name
3. Job names are case-sensitive

### "This branch is out-of-date"

**Cause**: Main branch has new commits since PR was created

**Solution**:
```bash
git checkout your-branch
git fetch origin
git rebase origin/main
git push --force-with-lease
```

### "Check has been running for too long"

**Cause**: Job timeout or hanging process

**Solution**:
1. Check Actions tab for details
2. Cancel and re-run the check
3. If persistent, investigate job logs

## Example Configuration

Here's a complete example of branch protection settings:

```yaml
Branch name pattern: main

Protect matching branches:
  ✓ Require a pull request before merging
    - Required approvals: 1
  ✓ Require status checks to pass before merging
    - Require branches to be up to date before merging
    - Status checks that are required:
      - Test (1.22)
      - Test (1.23)
      - Lint
      - Build (1.22)
      - Build (1.23)
      - Security Scan
  ✓ Require conversation resolution before merging
  ✓ Include administrators
  ✓ Restrict who can push to matching branches
    - Nobody (only via PR)
  ✓ Allow force pushes: Nobody
  ✓ Allow deletions: Nobody
```

## Additional Resources

- [GitHub Branch Protection Documentation](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/defining-the-mergeability-of-pull-requests/about-protected-branches)
- [GitHub Actions Status Checks](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/about-status-checks)
- [Best Practices for Branch Protection](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule)
