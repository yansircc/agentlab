package tool

const workerExtensionSource = `import { spawn } from "node:child_process";
import { Type } from "typebox";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

type Output = { content: { type: "text"; text: string }[]; details: {} };

export default async function (pi: ExtensionAPI) {
	const root = required("AGENTLAB_WORKER_FIXTURE");
	const executable = required("AGENTLAB_WORKER_DEPLOYCTL");
	const target = Type.String({ pattern: "^[a-z][a-z0-9-]{0,62}$" });
	const release = Type.String({ pattern: "^[a-z][a-z0-9-]{0,62}$" });
	pi.registerTool({ name: "deployctl_help", label: "deployctl help", description: "Show the public deployctl interface.", parameters: Type.Object({}), execute: async () => run(executable, root, ["help"]) });
	pi.registerTool({ name: "deployctl_deploy", label: "deployctl deploy", description: "Deploy one release to one validated target.", parameters: Type.Object({ target, release }), execute: async (_id, value) => run(executable, root, ["deploy", "--target", text(value, "target"), "--release", text(value, "release")]) });
	pi.registerTool({ name: "deployctl_status", label: "deployctl status", description: "Read one target's public deployment status.", parameters: Type.Object({ target }), execute: async (_id, value) => run(executable, root, ["status", "--target", text(value, "target")]) });
	pi.registerTool({ name: "deployctl_receipt", label: "deployctl receipt", description: "Read the public deployment receipt.", parameters: Type.Object({}), execute: async () => run(executable, root, ["receipt"]) });
	pi.on("session_start", () => pi.setActiveTools(["deployctl_help", "deployctl_deploy", "deployctl_status", "deployctl_receipt"]));
}

function required(key: string): string {
	const value = process.env[key];
	if (!value) throw new Error("Worker Host binding is unavailable.");
	return value;
}

function text(value: unknown, key: string): string {
	const result = typeof value === "object" && value !== null ? (value as Record<string, unknown>)[key] : undefined;
	if (typeof result !== "string" || !/^[a-z][a-z0-9-]{0,62}$/.test(result)) throw new Error("deployctl parameters are invalid.");
	return result;
}

function run(executable: string, root: string, args: string[]): Promise<Output> {
	return new Promise((resolve, reject) => {
		const child = spawn(executable, args, { env: { DEPLOYCTL_ROOT: root }, stdio: ["ignore", "pipe", "pipe"] });
		const stdout: Buffer[] = [], stderr: Buffer[] = [];
		child.stdout.on("data", (value: Buffer) => stdout.push(value));
		child.stderr.on("data", (value: Buffer) => stderr.push(value));
		child.once("error", () => reject(new Error("deployctl invocation failed.")));
		child.once("close", (code) => resolve({ content: [{ type: "text", text: JSON.stringify({ exit_code: code, stdout: redact(Buffer.concat(stdout).toString("utf8"), root), stderr: redact(Buffer.concat(stderr).toString("utf8"), root) }) }], details: {} }));
	});
}

function redact(value: string, root: string): string { return value.split(root).join("[fixture]"); }
`
