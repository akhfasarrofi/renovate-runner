// Global Renovate config. Keep this file in the renovate-runner project.
module.exports = {
  platform: "gitlab",
  endpoint: process.env.CI_API_V4_URL,
  globalExtends: ["config:recommended"],
  onboarding: false,
  requireConfig: "optional",
  automerge: false,
  // Begin with one pilot. Add more paths after validating its MRs.
  repositories: ["frontend/partner-v2"],
  baseBranchPatterns: ["$default"],
};
