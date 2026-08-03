import { readFileSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

const request = validate(JSON.parse(readFileSync(0, "utf8")));
const packageInfo = JSON.parse(readFileSync(join(request.package_root, "package.json"), "utf8"));
if (packageInfo.name !== request.package_name || packageInfo.version !== request.package_version) {
  throw new Error("Pi SDK package identity changed");
}
const sdk = await import(pathToFileURL(join(request.package_root, "dist", "index.js")).href);
const ai = await import(pathToFileURL(join(request.package_root, "node_modules", "@earendil-works", "pi-ai", "dist", "index.js")).href);
const parent = sdk.SessionManager.open(request.parent_session, request.child_session_dir);
const childPath = parent.createBranchedSession(request.entry_id);
if (!childPath) throw new Error("Pi SDK did not create a child session");
const resourceLoader = new sdk.DefaultResourceLoader({
  cwd: request.child_session_dir,
  agentDir: request.agent_dir,
  additionalExtensionPaths: [request.extension_path],
  noExtensions: true,
  noSkills: true,
  noPromptTemplates: true,
  noThemes: true,
  noContextFiles: true,
});
await resourceLoader.reload();
if (resourceLoader.getExtensions().errors.length !== 0 || resourceLoader.getExtensions().extensions.length !== 1) {
  throw new Error("Pi context filter extension did not load");
}
const faux = ai.fauxProvider();
let recalled = false;
faux.setResponses([(context) => {
  recalled = JSON.stringify(context).includes(request.private_token);
  return ai.fauxAssistantMessage(recalled ? "RECALLED" : "CLEAR");
}]);
const modelRuntime = await sdk.ModelRuntime.create();
modelRuntime.registerNativeProvider(faux.provider);
await modelRuntime.setRuntimeApiKey(faux.api, "faux");
const { session } = await sdk.createAgentSession({
  model: faux.getModel(), modelRuntime, resourceLoader, sessionManager: sdk.SessionManager.open(childPath, request.child_session_dir), tools: [], thinkingLevel: "off",
});
try {
  await session.prompt("continue");
} finally {
  session.dispose();
}
if (recalled) throw new Error("Pi context replayed private thinking");
process.stdout.write(`${JSON.stringify({ contract: "agentlab.pi-sdk-context.v1", private_recalled: false })}\n`);

function validate(value) {
  const required = ["package_root", "package_name", "package_version", "parent_session", "child_session_dir", "agent_dir", "extension_path", "entry_id", "private_token"];
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).length !== required.length || required.some((key) => typeof value[key] !== "string" || value[key] === "")) {
    throw new Error("Pi context request is invalid");
  }
  if (Object.keys(value).some((key) => !required.includes(key))) throw new Error("Pi context request has unknown fields");
  return value;
}
