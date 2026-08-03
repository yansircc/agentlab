import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";
import { join } from "node:path";

const input = readRequest();
const request = validateRequest(input);
const packageJSON = JSON.parse(readFileSync(join(request.package_root, "package.json"), "utf8"));
if (packageJSON.name !== request.package_name || packageJSON.version !== request.package_version) {
  throw new Error("Pi SDK package identity changed");
}
const sdk = await import(pathToFileURL(join(request.package_root, "dist", "index.js")).href);
if (typeof sdk.SessionManager !== "function") throw new Error("Pi SDK SessionManager is unavailable");
const parent = sdk.SessionManager.open(request.parent_session, request.child_session_dir);
if (!parent.getEntry(request.entry_id)) throw new Error("Pi fork entry does not exist");
const parentSessionID = parent.getSessionId();
const childSession = parent.createBranchedSession(request.entry_id);
if (!childSession) throw new Error("Pi SDK did not create a child session");
const child = sdk.SessionManager.open(childSession, request.child_session_dir);
write({
  contract: "agentlab.pi-sdk-fork.v1",
  parent_session_id: parentSessionID,
  child_session_id: child.getSessionId(),
  child_session_path: childSession,
  child_leaf_id: child.getLeafId(),
});

function readRequest() {
  const data = readFileSync(0, "utf8");
  if (!data || data.trim() !== data.trimEnd()) throw new Error("Pi SDK request is invalid");
  return JSON.parse(data);
}

function validateRequest(value) {
  const required = ["package_root", "package_name", "package_version", "parent_session", "child_session_dir", "entry_id"];
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).length !== required.length || required.some((key) => typeof value[key] !== "string" || value[key] === "")) {
    throw new Error("Pi SDK request is invalid");
  }
  if (Object.keys(value).some((key) => !required.includes(key))) throw new Error("Pi SDK request has unknown fields");
  return value;
}

function write(value) {
  process.stdout.write(`${JSON.stringify(value)}\n`);
}
