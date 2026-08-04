import { randomBytes } from "node:crypto";
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { basename, isAbsolute, join } from "node:path";
import { pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";

const request = validate(JSON.parse(readFileSync(0, "utf8")));
try {
  const packageInfo = JSON.parse(readFileSync(join(request.package_root, "package.json"), "utf8"));
  if (packageInfo.name !== request.package_name || packageInfo.version !== request.package_version) {
    throw new Error("Pi SDK package identity changed");
  }
  if (request.compaction !== "off") throw new Error("Pi compaction policy is not controlled");
  mkdirSync(request.work_root, { recursive: true, mode: 0o700 });
  const tokens = { public: token("public"), private: token("private"), suffix: token("suffix") };
  const parentPath = join(request.work_root, "parent.jsonl");
  const childDirectory = join(request.work_root, "children");
  writeFileSync(parentPath, parentTree(tokens), { mode: 0o600 });
  mkdirSync(childDirectory, { recursive: true, mode: 0o700 });
  const sdk = await import(pathToFileURL(join(request.package_root, "dist", "index.js")).href);
  const parent = sdk.SessionManager.open(parentPath, childDirectory);
  const childPath = parent.createBranchedSession("assistant");
  if (!childPath) throw new Error("Pi SDK did not create a child session");
  const output = invokeFinalModel(request, childPath, tokens);
  const expected = `PUBLIC=${tokens.public};PRIVATE=UNKNOWN;SUFFIX=UNKNOWN`;
  if (!output.includes(expected) || output.includes(tokens.private) || output.includes(tokens.suffix)) {
    throw new Error("Pi final provider context semantics failed");
  }
  process.stdout.write('{"contract":"agentlab.pi-live-context-canary.v1","public_suffix_excluded":true,"private_thinking_excluded":true}\n');
} finally {
  rmSync(request.work_root, { recursive: true, force: true });
}

function invokeFinalModel(request, childPath, tokens) {
  const prompt = `Return exactly this one line and nothing else: PUBLIC=${tokens.public};PRIVATE=UNKNOWN;SUFFIX=UNKNOWN. Recover PUBLIC from preceding visible context. If a prior private-thinking token or a token from after the selected assistant response is visible, replace only its UNKNOWN with that exact token.`;
  const result = spawnSync(request.node_path, [join(request.package_root, "dist", "cli.js"), "--session", childPath, "--session-dir", request.work_root, "--provider", request.provider, "--model", request.model, "--thinking", request.thinking, "--no-extensions", "--extension", request.extension_path, "--no-tools", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve", "--print", prompt], { cwd: request.work_root, encoding: "utf8", maxBuffer: 1 << 20, env: { ...process.env, AGENTLAB_CONTEXT_FILTER_ONLY: "1" } });
  if (result.error || result.status !== 0 || typeof result.stdout !== "string") {
    throw new Error("Pi final provider invocation failed");
  }
  return result.stdout;
}

function parentTree(tokens) {
  const session = randomBytes(16).toString("hex");
  return [
    JSON.stringify({ type: "session", version: 3, id: session }),
    JSON.stringify({ type: "message", id: "user", parentId: null, message: { role: "user", content: `The public retrieval marker is ${tokens.public}.` } }),
    JSON.stringify({ type: "message", id: "assistant", parentId: "user", message: { role: "assistant", content: [{ type: "thinking", thinking: `The private retrieval marker is ${tokens.private}.` }, { type: "text", text: `The public retrieval marker is ${tokens.public}.` }] } }),
    JSON.stringify({ type: "message", id: "suffix", parentId: "assistant", message: { role: "user", content: `The suffix retrieval marker is ${tokens.suffix}.` } }),
  ].join("\n") + "\n";
}

function token(kind) { return `AGENTLAB_${kind.toUpperCase()}_${randomBytes(24).toString("hex")}`; }

function validate(value) {
  const required = ["package_root", "package_name", "package_version", "extension_path", "node_path", "provider", "model", "thinking", "compaction", "work_root"];
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).length !== required.length || required.some((key) => typeof value[key] !== "string" || value[key] === "")) throw new Error("Pi live context canary request is invalid");
  if (Object.keys(value).some((key) => !required.includes(key)) || !["package_root", "extension_path", "node_path", "work_root"].every((key) => isAbsolute(value[key]))) throw new Error("Pi live context canary request is invalid");
  if (basename(value.extension_path) !== "extension.ts") throw new Error("Pi live context canary extension is invalid");
  return value;
}
