// Global Renovate config. Keep this file in the renovate-runner project.
// Testing: set RENOVATE_TEST_REPOSITORY=group/project to run on that one project only.
// Production: RENOVATE_TARGET_REPOSITORIES lists GitLab groups, e.g. "frontend,backend".
// Renovate then runs on every project inside those groups (subgroups: "frontend/web").
const testRepository = (process.env.RENOVATE_TEST_REPOSITORY || "").trim();
const groups = (process.env.RENOVATE_TARGET_REPOSITORIES || "")
  .split(/[\n,]/)
  .map((group) => group.trim())
  .filter(Boolean);

if (testRepository && !testRepository.includes("/")) {
  throw new Error("RENOVATE_TEST_REPOSITORY must be a group/project path");
}
if (
  !testRepository &&
  (!groups.length || new Set(groups).size !== groups.length)
) {
  throw new Error(
    "RENOVATE_TARGET_REPOSITORIES must list unique GitLab group names",
  );
}

// The platform filters by group, so only these groups' projects are listed.
const targets = testRepository
  ? { repositories: [testRepository] }
  : { autodiscover: true, autodiscoverNamespaces: groups };

module.exports = {
  platform: "gitlab",
  endpoint: process.env.CI_API_V4_URL,
  globalExtends: ["config:recommended", "mergeConfidence:all-badges"],
  onboarding: false,
  requireConfig: "optional",
  automerge: false,
  // OSV alerts generate security-fix MRs when a direct dependency has a fix.
  osvVulnerabilityAlerts: true,
  ...targets,
  baseBranchPatterns: ["$default"],
};
