package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWorkflowNextSteps(t *testing.T) {
	response := fakeStatusResponse()
	root := t.TempDir()
	state := root + "/.azurpromilia"

	cases := []struct {
		command string
		want    []string
	}{
		{"install", []string{"安装完成", "verify", "PROTON_SETUP"}},
		{"update", []string{"游戏已更新", "verify"}},
		{"repair", []string{"修复完成", "verify"}},
		{"preload", []string{"预下载已准备", "update"}},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			var b bytes.Buffer
			printWorkflowNextSteps(&b, tc.command, root, state, response)
			out := b.String()
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("%s guidance missing %q:\n%s", tc.command, want, out)
				}
			}
		})
	}
}

func TestVerifyNextSteps(t *testing.T) {
	var b bytes.Buffer
	printVerifyNextSteps(&b, "released", "1.0.2")
	if out := b.String(); !strings.Contains(out, "校验通过") || !strings.Contains(out, "启动游戏") {
		t.Fatalf("released verify guidance=%q", out)
	}

	b.Reset()
	printVerifyNextSteps(&b, "preload", "1.0.3")
	if out := b.String(); !strings.Contains(out, "_pre_download") || !strings.Contains(out, "update") {
		t.Fatalf("preload verify guidance=%q", out)
	}
}
