// Global Renovate config. Keep this file in the renovate-runner project.
// RENOVATE_PLATFORM: gitlab (default), github or bitbucket.
// RENOVATE_TARGET_REPOSITORY, comma-separated, one of:
//  - owner/repo[,owner/repo2]  only those repositories
//  - group[,group2]            every repository in the GitLab group / GitHub org / Bitbucket workspace
// An entry with "/" is a repository, so GitLab subgroups cannot be used as a target.
const platform = (process.env.RENOVATE_PLATFORM || "gitlab").trim();
if (!["gitlab", "github", "bitbucket"].includes(platform)) {
  throw new Error("RENOVATE_PLATFORM must be gitlab, github or bitbucket");
}
const targetList = (process.env.RENOVATE_TARGET_REPOSITORY || "")
  .split(/[\n,]/)
  .map((item) => item.trim())
  .filter(Boolean);
const repositories = targetList.filter((item) => item.includes("/"));
const groups = targetList.filter((item) => !item.includes("/"));

if (!targetList.length || new Set(targetList).size !== targetList.length) {
  throw new Error(
    "RENOVATE_TARGET_REPOSITORY must list unique owner/repo paths or group names",
  );
}
if (repositories.length && groups.length) {
  throw new Error(
    "RENOVATE_TARGET_REPOSITORY must list either owner/repo paths or group names, not both",
  );
}

// GitLab filters by group on the server side. Renovate documents autodiscoverNamespaces
// only for GitLab/Gitea/Forgejo, so GitHub and Bitbucket use a glob filter instead.
const discovery =
  platform === "gitlab"
    ? { autodiscoverNamespaces: groups }
    : { autodiscoverFilter: groups.map((group) => `${group}/*`) };
const targets = repositories.length
  ? { repositories }
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

  // Batasi noise: MR baru per jam / terbuka bersamaan
  prHourlyLimit: 4,
  prConcurrentLimit: 10,

  // Security fixes should not be queued behind the normal limit
  vulnerabilityAlerts: { prConcurrentLimit: 5 },

  // Wait for a "mature" release (to avoid packages that have just been published and subsequently pulled, or those containing malware)
  minimumReleaseAge: "3 days",

  // Hanya rebase saat konflik, bukan tiap base branch bergerak
  rebaseWhen: "conflicted",

  labels: ["dependencies"],
  commitMessagePrefix: "chore(deps):",
  semanticCommits: "disabled",

  // Reduce MR: group minor/patch changes together
  packageRules: [
    {
      matchUpdateTypes: ["minor", "patch"],
      matchCurrentVersion: "!/^0/",
      groupName: "non-major dependencies",
    },
    { matchUpdateTypes: ["major"], addLabels: ["major"] },
  ],
};
