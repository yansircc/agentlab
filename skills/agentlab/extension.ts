import { execFileSync, spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { Type } from "typebox";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

type Definition = { name: string; description: string; input_schema: Record<string, unknown> };
type Output = { ok: boolean; data?: unknown };
type Binding = { args: string[]; hidden: string[] };

export default async function (pi: ExtensionAPI) {
	const binary = fileURLToPath(new URL("./bin/agentlab", import.meta.url));
	const active = definitions(binary);
	pi.on("context", (event) => ({ messages: event.messages.flatMap(publicMessage) }));
	for (const definition of active) {
		pi.registerTool({
			name: definition.name,
			label: definition.name,
			description: definition.description,
			parameters: Type.Unsafe<Record<string, unknown>>(definition.input_schema),
			execute: async (_id, params, signal) => {
				const binding = hostBinding();
				const result = await invoke(binary, [...binding.args, "-name", definition.name], JSON.stringify(params), signal);
				if (!result.ok) throw new Error("AgentLab operation was rejected.");
				const text = JSON.stringify(result.data ?? null);
				if (binding.hidden.some((value) => value !== "" && text.includes(value))) {
					throw new Error("AgentLab result contains an unavailable Host locator.");
				}
				return { content: [{ type: "text", text }], details: {} };
			},
		});
	}
	pi.on("session_start", () => pi.setActiveTools(active.map((definition) => definition.name)));
}

function publicMessage(message: { role: string; content?: unknown }): typeof message[] {
	if (message.role !== "assistant" || !Array.isArray(message.content)) return [message];
	const content = message.content.filter((block) => !(block && typeof block === "object" && (block as { type?: unknown }).type === "thinking"));
	return content.length === 0 ? [] : [{ ...message, content }];
}

function definitions(binary: string): Definition[] {
	try {
		const value = decode(execFileSync(binary, ["tool", "schemas", "-provider", "anthropic"], { encoding: "utf8", maxBuffer: 1 << 20 }));
		if (!value.ok || !Array.isArray(value.data) || value.data.length !== 4) throw new Error();
		const result = value.data as Definition[];
		if (new Set(result.map((item) => item.name)).size !== result.length || result.some((item) => !validDefinition(item))) throw new Error();
		return result;
	} catch {
		throw new Error("AgentLab bundled schema projection is unavailable.");
	}
}

function validDefinition(value: Definition): boolean {
	return typeof value?.name === "string" && value.name.startsWith("agentlab_") && typeof value.description === "string" && value.input_schema !== null && typeof value.input_schema === "object";
}

function hostBinding(): Binding {
	const root = process.env.AGENTLAB_ROOT;
	if (!root) throw new Error("AgentLab Host binding is unavailable.");
	const args = ["tool", "invoke", "-root", root];
	const hidden = [root];
	for (const [environment, flag] of [["AGENTLAB_PREPARATION", "-preparation"], ["AGENTLAB_EXPERIMENT", "-experiment"], ["AGENTLAB_PI_RUNTIME_PLAN", "-pi-runtime-plan"]]) {
		const value = process.env[environment];
		if (value) {
			args.push(flag, value);
			hidden.push(value);
		}
	}
	return { args, hidden };
}

function invoke(binary: string, args: string[], input: string, signal: AbortSignal | undefined): Promise<Output> {
	return new Promise((resolve, reject) => {
		const child = spawn(binary, args, { stdio: ["pipe", "pipe", "ignore"] });
		const output: Buffer[] = [];
		const abort = () => child.kill("SIGTERM");
		signal?.addEventListener("abort", abort, { once: true });
		child.stdout.on("data", (chunk: Buffer) => output.push(chunk));
		child.once("error", () => reject(new Error("AgentLab invocation failed.")));
		child.once("close", () => {
			signal?.removeEventListener("abort", abort);
			try {
				resolve(decode(Buffer.concat(output).toString("utf8")));
			} catch {
				reject(new Error("AgentLab invocation failed."));
			}
		});
		child.stdin.end(input);
	});
}

function decode(data: string): Output {
	const value = JSON.parse(data) as Output;
	if (typeof value?.ok !== "boolean") throw new Error("invalid AgentLab output");
	return value;
}
