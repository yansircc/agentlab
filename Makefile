DIST := dist/skill
SOURCE := skills/agentlab
PI ?= pi

.PHONY: skill

skill:
	test ! -L dist
	test ! -L "$(DIST)"
	rm -rf "$(DIST)"
	mkdir -p "$(DIST)/bin"
	cp "$(SOURCE)/SKILL.md" "$(DIST)/SKILL.md"
	cp "$(SOURCE)/extension.ts" "$(DIST)/extension.ts"
	go build -trimpath -o "$(DIST)/bin/agentlab" ./cmd/agentlab
	@skill_tmp="$$(mktemp -d /tmp/agentlab-skill.XXXXXX)"; trap 'rm -rf "$$skill_tmp"' EXIT; go build -trimpath -o "$$skill_tmp/agentlab" ./cmd/agentlab; node scripts/skill-contract.mjs --dist "$(DIST)" --source "$(SOURCE)" --comparison-binary "$$skill_tmp/agentlab" --pi "$(PI)"
	go test -count=1 ./internal/adapter/pi -run 'TestPinnedSDKForksExactAssistantPublicPrefix|TestDiscoverIdentityBindsInstalledSDKAndBridge|TestPublicTreeExcludesThinkingAndClosesToolCausality|TestPinnedSDKDoesNotReplayPrivateThinkingToFauxModel'
