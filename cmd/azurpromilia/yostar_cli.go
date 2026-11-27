package main

import (
 "bufio"
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "strconv"
 "strings"
)

func printYostarHelp(w io.Writer, prog string) {
 fmt.Fprintf(w, "Yostar JP — EXPERIMENTAL / full installation unverified\n\nUsage: %s yostar <info|status|resources|versions|install|verify|repair|sync|probe|path> [options]\n  -p, --path PATH   Separate remembered JP game directory\n  -j, --workers N   Parallel workers (1..32)\n  --launcher-version VERSION   Override official launcher version\n\nExample: %s yostar -p \"$HOME/Games/AzurPromilia-JP\" install\nNo Wine or Windows launcher required for JP resource download.\n",prog,prog)
}
func rejectYostarRoot(path string) error {
 if _,e:=os.Lstat(filepath.Join(path,jpState));e==nil{return errors.New("Yostar JP state found: use a separate SC000 directory")}else if !os.IsNotExist(e){return e};return nil
}
func jpDistinctPath(p, cn string) error {
 if cn=="" {return nil}
 a,e:=filepath.Abs(p);if e!=nil{return e};b,e:=filepath.Abs(cn);if e!=nil{return e}
 if x,e:=filepath.EvalSymlinks(a);e==nil{a=x};if x,e:=filepath.EvalSymlinks(b);e==nil{b=x}
 if a==b||strings.HasPrefix(a,b+string(os.PathSeparator))||strings.HasPrefix(b,a+string(os.PathSeparator)){return errors.New("JP and SC000 game directories must be separate")}
 return nil
}
func rememberYostarPath(p string) (string,error) {
 if strings.TrimSpace(p)==""{return "",errors.New("empty JP game path")}
 abs,e:=filepath.Abs(p);if e!=nil{return "",e}
 cfg,e:=loadCLIConfig();if e!=nil{return "",e}
 if e=jpDistinctPath(abs,cfg.GamePath);e!=nil{return "",e}
 for _,n:=range []string{".azurpromilia","_index","_pre_download"}{if _,e:=os.Lstat(filepath.Join(abs,n));e==nil{return "",errors.New("SC000 state found; use another JP directory")}else if !os.IsNotExist(e){return "",e}}
 cfg.YostarPath=abs;return abs,saveCLIConfig(cfg)
}
func runYostarCLI(opts commonCLIOptions,args []string) error {
 if len(args)==0||args[0]=="help"{printYostarHelp(os.Stdout,cliProgramName());return nil}
 cmd:=args[0]
 switch cmd {case "sync":cmd="repair";case "versions":cmd="resources"}
 switch cmd {case "path","info","status","resources","install","repair","verify","probe":default:return fmt.Errorf("unknown Yostar command %q",cmd)}
 cfg,e:=loadCLIConfig();if e!=nil{return e}
 p:=cfg.YostarPath
 if cmd=="path" {
  if len(args)>2 || (len(args)==2&&opts.Path!=""){return errors.New("usage: azp yostar path [DIRECTORY]")}
  if len(args)==2{opts.Path=args[1]}
  if opts.Path!=""{p,e=rememberYostarPath(opts.Path);if e!=nil{return e}}
  if p==""{return errors.New("no JP game directory configured; use azp yostar path DIRECTORY")}
  fmt.Println(p);return nil
 }
 if opts.Path!=""{p,e=rememberYostarPath(opts.Path);if e!=nil{return e}}
 if p==""{home,e:=os.UserHomeDir();if e!=nil{return e};p=filepath.Join(home,"Games","AzurPromilia-JP")}
 if e=jpDistinctPath(p,cfg.GamePath);e!=nil{return e}
 workers:=opts.Workers;if workers==""{workers=strconv.Itoa(preferredWorkers(cfg))}
 if !opts.Quiet {fmt.Fprintln(os.Stderr,"WARNING: Yostar JP is experimental; full installation and launch unverified.")}
 return runYostar(append(append([]string{cmd},args[1:]...),"--root",p,"--workers",workers))
}
func runYostarMenu() error {
 for {
  cfg,e:=loadCLIConfig();if e!=nil{return e}
  action,e:=chooseSimpleMenu("Yostar JP（实验性）","全量安装与启动尚未验证；独立目录："+cfg.YostarPath,[]menuItem{
   {Key:"status",Label:"状态 / 资源",Description:"读取官方版本及文件清单"},
   {Key:"install",Label:"安装",Description:"下载正式版（未验证全量安装）"},
   {Key:"verify",Label:"校验",Description:"按本地清单校验"},
   {Key:"sync",Label:"修复",Description:"补齐缺失或损坏文件（未实测）"},
   {Key:"probe",Label:"下载探针",Description:"下载一个小型官方资源"},
   {Key:"path",Label:"日服目录",Description:"单独设置，不影响国服"},
   {Key:"back",Label:"返回",Description:"返回国服主菜单"},
  },0)
  if e!=nil||action==""||action=="back"{return e}
  if action=="path"{
   home,e:=os.UserHomeDir();if e!=nil{return e};def:=filepath.Join(home,"Games","AzurPromilia-JP")
   fmt.Printf("日服目录 [%s]: ",def);line,e:=bufio.NewReader(os.Stdin).ReadString('\n');if e!=nil{return e};p:=strings.TrimSpace(line);if p==""{p=def};if _,e=rememberYostarPath(p);e!=nil{return e};continue
  }
  return runCLI([]string{"yostar",action})
 }
}
