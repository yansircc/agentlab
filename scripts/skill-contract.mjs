import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdtempSync, readFileSync, readdirSync, realpathSync, renameSync, rmSync, statSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { pathToFileURL } from "node:url";

const options = parse(process.argv.slice(2));
const dist = resolve(required(options, "dist"));
const source = resolve(required(options, "source"));
const comparison = resolve(required(options, "comparison-binary"));
const pi = required(options, "pi");
const binary = join(dist, "bin", "agentlab");

assert.deepEqual(files(dist), ["SKILL.md", "bin/agentlab", "extension.ts"]);
assert.equal(readFileSync(join(source, "SKILL.md"), "utf8"), readFileSync(join(dist, "SKILL.md"), "utf8"));
assert.equal(readFileSync(join(source, "extension.ts"), "utf8"), readFileSync(join(dist, "extension.ts"), "utf8"));
assert.match(readFileSync(join(dist, "SKILL.md"), "utf8"), /^---\nname: agentlab\n[\s\S]*executables:\n  agentlab: bin\/agentlab\n---\n/);
assert.equal(statSync(binary).mode & 0o111, 0o111);
assert.equal(digest(binary), digest(comparison));

const definitions = schemas(binary);
const sourceExtension = readFileSync(join(source, "extension.ts"), "utf8");
for (const definition of definitions) assert.equal(sourceExtension.includes(definition.name), false, "extension copied a tool name");
assert.equal(readFileSync(join(source, "SKILL.md"), "utf8").includes("JSONL"), false, "Skill mentions a Pi private format");

await assertExtension(definitions);
console.log(JSON.stringify({ ok: true, tools: definitions.map((value) => value.name), binary_sha256: digest(binary) }));

function parse(args) {
	const result = {};
	for (let index = 0; index < args.length; index += 2) {
		assert.ok(args[index]?.startsWith("--") && args[index + 1]);
		result[args[index].slice(2)] = args[index + 1];
	}
	return result;
}

function required(options, key) {
	assert.equal(typeof options[key], "string", `missing --${key}`);
	return options[key];
}

function files(root, prefix = "") {
	return readdirSync(root, { withFileTypes: true }).flatMap((entry) => {
		const path = join(root, entry.name);
		const relative = join(prefix, entry.name);
		if (entry.isDirectory()) return files(path, relative);
		assert.ok(entry.isFile(), `unexpected artifact entry ${relative}`);
		return [relative];
	}).sort();
}

function digest(path) {
	return createHash("sha256").update(readFileSync(path)).digest("hex");
}

function schemas(executable) {
	const value = JSON.parse(execFileSync(executable, ["tool", "schemas", "-provider", "anthropic"], { encoding: "utf8" }));
	assert.equal(value.ok, true);
	assert.ok(Array.isArray(value.data) && value.data.length === 4);
	const names = value.data.map((item) => item.name);
	assert.equal(new Set(names).size, 4);
	assert.ok(names.every((name) => typeof name === "string" && name.startsWith("agentlab_")));
	assert.ok(value.data.every((item) => item.input_schema && typeof item.input_schema === "object"));
	assert.doesNotMatch(JSON.stringify(value.data), /request_path|stream_path|session_path|executable_path|raw_transcript|audit_root/);
	return value.data;
}

async function assertExtension(expected) {
	const piRoot = resolvePiRoot(pi);
	const packageInfo = JSON.parse(readFileSync(join(piRoot, "package.json"), "utf8"));
	assert.deepEqual([packageInfo.name, packageInfo.version], ["@earendil-works/pi-coding-agent", "0.83.0"]);
	const loader = await import(pathToFileURL(join(piRoot, "dist", "core", "extensions", "loader.js")).href);
	const root = mkdtempSync(join(tmpdir(), "agentlab-extension-"));
	const saved = new Map(["AGENTLAB_ROOT", "AGENTLAB_PREPARATION", "AGENTLAB_EXPERIMENT", "AGENTLAB_PI_RUNTIME_PLAN", "AGENTLAB_EXECUTABLE"].map((key) => [key, process.env[key]]));
	process.env.AGENTLAB_ROOT = root;
	delete process.env.AGENTLAB_PREPARATION;
	delete process.env.AGENTLAB_EXPERIMENT;
	delete process.env.AGENTLAB_PI_RUNTIME_PLAN;
	process.env.AGENTLAB_EXECUTABLE = "/must-not-be-used";
	try {
		const { registered, active } = await register(loader);
		assert.deepEqual(registered.map((tool) => tool.name), expected.map((tool) => tool.name));
		assert.deepEqual(active, expected.map((tool) => tool.name));
		for (let index = 0; index < expected.length; index += 1) assert.deepEqual(registered[index].parameters, expected[index].input_schema);
		const inspect = registered.find((tool) => tool.name.endsWith("inspect"));
		await assert.rejects(() => inspect.execute("contract", { scope: "preparation", after: 0, limit: 1 }), (error) => error instanceof Error && error.message.startsWith("AgentLab operation was rejected"));
		const moved = `${binary}.contract-hold`;
		renameSync(binary, moved);
		try {
			const unavailable = await loader.loadExtensions([join(dist, "extension.ts")], process.cwd());
			assert.equal(unavailable.extensions.length, 0);
			assert.equal(unavailable.errors.length, 1);
			assert.match(unavailable.errors[0].error, /AgentLab bundled schema projection is unavailable/);
		} finally {
			renameSync(moved, binary);
		}
	} finally {
		for (const [key, value] of saved) {
			if (value === undefined) delete process.env[key];
			else process.env[key] = value;
		}
		rmSync(root, { recursive: true, force: true });
	}
}

function resolvePiRoot(command) {
	const executable = execFileSync("which", [command], { encoding: "utf8" }).trim();
	assert.ok(executable);
	const root = dirname(dirname(realpathSync(executable)));
	assert.ok(existsSync(join(root, "package.json")), "Pi package root is unavailable");
	return root;
}

async function register(loader) {
	let active;
	const loaded = await loader.loadExtensions([join(dist, "extension.ts")], process.cwd());
	assert.deepEqual(loaded.errors, []);
	assert.equal(loaded.extensions.length, 1);
	const extension = loaded.extensions[0];
	const tools = [...extension.tools.values()].map((value) => value.definition);
	const context = extension.handlers.get("context") ?? [];
	assert.equal(context.length, 1);
	const visible = await context[0]({ type: "context", messages: [{ role: "assistant", content: [{ type: "thinking", thinking: "private" }, { type: "text", text: "public" }] }] }, {});
	assert.deepEqual(visible, { messages: [{ role: "assistant", content: [{ type: "text", text: "public" }] }] });
	loaded.runtime.setActiveTools = (value) => { active = value; };
	const handlers = extension.handlers.get("session_start") ?? [];
	assert.equal(handlers.length, 1);
	await handlers[0]({ type: "session_start" }, {});
	return { registered: tools, active };
}
