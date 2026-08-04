import source from "node:fs";

const text = source.readFileSync(new URL("./extension.ts", import.meta.url), "utf8");
const filter = text.indexOf('pi.on("context"');
const gate = text.indexOf('AGENTLAB_CONTEXT_FILTER_ONLY');
const tools = text.indexOf("const active = definitions");
if (filter < 0 || gate < filter || tools < gate) throw new Error("context filter must load before the tool authority gate");
