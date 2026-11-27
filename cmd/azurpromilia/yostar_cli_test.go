package main

import (
 "bytes"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestYostarSeparateRememberedDirectory(t *testing.T) {
 t.Setenv("XDG_CONFIG_HOME", t.TempDir())
 root:=t.TempDir()
 cn,jp:=filepath.Join(root,"CN"),filepath.Join(root,"JP")
 if _,e:=rememberGamePath(cn);e!=nil{t.Fatal(e)}
 if _,e:=rememberYostarPath(jp);e!=nil{t.Fatal(e)}
 if e:=setPreferredWorkers(8);e!=nil{t.Fatal(e)}
 cfg,e:=loadCLIConfig();if e!=nil{t.Fatal(e)}
 if cfg.GamePath!=cn||cfg.YostarPath!=jp||preferredWorkers(cfg)!=8{t.Fatalf("bad config %+v",cfg)}
 if _,e=rememberYostarPath(cn);e==nil{t.Fatal("JP accepted CN path")}
 if _,e=rememberGamePath(jp);e==nil{t.Fatal("CN accepted JP path")}
 if _,e=rememberGamePath(filepath.Join(jp,"nested"));e==nil{t.Fatal("CN accepted nested JP path")}
 if e=os.MkdirAll(filepath.Join(root,"oldJP",jpState),0755);e!=nil{t.Fatal(e)}
 if _,e=rememberGamePath(filepath.Join(root,"oldJP"));e==nil{t.Fatal("CN accepted existing JP state")}
 cfg,e=loadCLIConfig();if e!=nil||cfg.GamePath!=cn||cfg.YostarPath!=jp{t.Fatalf("rejected path mutated config: %+v %v",cfg,e)}
}
func TestYostarPreviewEntryPoints(t *testing.T) {
 var b bytes.Buffer
 printYostarHelp(&b,"azp")
 if !strings.Contains(b.String(),"EXPERIMENTAL")||!strings.Contains(b.String(),"unverified"){t.Fatal("missing preview warning")}
 items,_:=buildMainMenu("")
 found:=false
 for _,item:=range items{if item.Key=="yostar"&&strings.Contains(item.Label,"实验性"){found=true}}
 if !found{t.Fatal("missing preview menu")}
 for _,shell:=range []string{"bash","zsh","fish"}{s,e:=completionScript(shell);if e!=nil||!strings.Contains(s,"yostar"){t.Fatalf("missing %s completion",shell)}}
}
