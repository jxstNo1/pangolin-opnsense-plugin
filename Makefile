.PHONY: all agent test lint dist clean

VERSION ?= 0.1.0

all: test agent

agent:
	VERSION=$(VERSION) sh scripts/build-agent.sh

test:
	cd agent && go test ./...

lint:
	sh scripts/lint.sh

dist: agent
	VERSION=$(VERSION) sh scripts/build-plugin.sh

clean:
	rm -rf dist
	rm -f plugin/src/sbin/pangolin-agent
