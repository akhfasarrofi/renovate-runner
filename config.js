// Global Renovate config. Keep this file in the renovate-runner project.
// RENOVATE_PLATFORM: gitlab (default), github or bitbucket.
// Testing: set RENOVATE_TEST_REPOSITORY=owner/repo[,owner/repo2] to run on those repositories only.
// Production: RENOVATE_TARGET_REPOSITORIES lists GitLab groups, GitHub orgs/users or
// Bitbucket workspaces, e.g. "frontend,backend". Renovate then runs on every repository
// inside them (GitLab subgroups: "frontend/web").
const platform = (process.env.RENOVATE_PLATFORM || "gitlab").trim();
if (!["gitlab", "github", "bitbucket"].includes(platform)) {
  throw new Error("RENOVATE_PLATFORM must be gitlab, github or bitbucket");
}
const testRepositories = (process.env.RENOVATE_TEST_REPOSITORY || "")
  .split(/[\n,]/)
  .map((repository) => repository.trim())
  .filter(Boolean);
const groups = (process.env.RENOVATE_TARGET_REPOSITORIES || "")
  .split(/[\n,]/)
  .map((group) => group.trim())
  .filter(Boolean);

if (testRepositories.some((repository) => !repository.includes("/"))) {
  throw new Error("RENOVATE_TEST_REPOSITORY must list owner/repo paths");
}
if (
  !testRepositories.length &&
  (!groups.length || new Set(groups).size !== groups.length)
) {
  throw new Error(
    "RENOVATE_TARGET_REPOSITORIES must list unique group/org/workspace names",
  );
}

// GitLab filters by group on the server side. Renovate documents autodiscoverNamespaces
// only for GitLab/Gitea/Forgejo, so GitHub and Bitbucket use a glob filter instead.
const discovery =
  platform === "gitlab"
    ? { autodiscoverNamespaces: groups }
    : { autodiscoverFilter: groups.map((group) => `${group}/*`) };
const targets = testRepositories.length
  ? { repositories: testRepositories }
  : { autodiscover: true, ...discovery };

module.exports = {
  platform,
  // GitLab CI provides its API URL; elsewhere use RENOVATE_ENDPOINT or Renovate's default.
  ...(platform === "gitlab" && process.env.CI_API_V4_URL
    ? { endpoint: process.env.CI_API_V4_URL }
    : {}),
  globalExtends: ["config:recommended", "mergeConfidence:all-badges"],
  onboarding: false,
  requireConfig: "optional",
  automerge: false,
  // OSV alerts generate security-fix MRs when a direct dependency has a fix.
  osvVulnerabilityAlerts: true,
  ...targets,
  baseBranchPatterns: ["$default"],
};
