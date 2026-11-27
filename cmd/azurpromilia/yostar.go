package main

import (
    "context"
    "crypto/md5"
    "encoding/hex"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "hash/crc64"
    "io"
    "net/http"
    "net/url"
    "os"
    "os/signal"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

// Yostar's distributed desktop launcher uses an anonymous, public client
// metadata signature, not a user credential. No login or account API is used.
const jpAPI = "https://api-launcher-jp.yo-star.com"
const jpVersion = "1.8.1"
const jpPublicSalt = "DE7108E9B2842FD460F4777702727869"
const jpState = ".azurpromilia-yostar"
var jpCRCTable = crc64.MakeTable(crc64.ECMA)

func validEntryChecksum(s string) bool {
    if strings.HasPrefix(s, "crc64:") {
        n, err := strconv.ParseUint(strings.TrimPrefix(s, "crc64:"), 10, 64)
        return err == nil && s == "crc64:"+strconv.FormatUint(n, 10)
    }
    return validMD5(s)
}
func verifyEntryHash(r io.Reader, checksum string) bool {
    if strings.HasPrefix(checksum, "crc64:") {
        h := crc64.New(jpCRCTable)
        _, err := io.Copy(h, r)
        return err == nil && checksum == "crc64:"+strconv.FormatUint(h.Sum64(),10)
    }
    h := md5.New()
    _, err := io.Copy(h,r)
    return err == nil && strings.EqualFold(hex.EncodeToString(h.Sum(nil)),checksum)
}

func jpAuth(version string, now int64) string {
    head := struct {
        GameTag string `json:"game_tag"`
        Time int64 `json:"time"`
        Version string `json:"version"`
    }{"AZUPRO_JP", now, version}
    b,_ := json.Marshal(head)
    sum:=md5.Sum(append(b, []byte(jpPublicSalt)...))
    out,_:=json.Marshal(struct{
        Head json.RawMessage `json:"head"`
        Sign string `json:"sign"`
    }{b,hex.EncodeToString(sum[:])})
    return string(out)
}
type jpGameConfig struct {
    Version string `json:"game_latest_version"`
    Basis string `json:"game_latest_file_path"`
    LaunchExe string `json:"game_start_exe_name"`
}
type jpDomains struct {
    Primary string `json:"primary_cdn"`
    Backup string `json:"back_up_cdn"`
}
type jpManifestURL struct { URL string `json:"url"` }
type jpRawFile struct {
    Path string `json:"path"`
    Size json.RawMessage `json:"size"`
    Hash json.RawMessage `json:"hash"`
}
type jpRawManifest struct {
    Source string `json:"source"`
    File []jpRawFile `json:"file"`
}
func jpHTTPS(raw string) (*url.URL,error) {
    u,e:=url.Parse(raw)
    if e!=nil || u.Scheme!="https" || u.Hostname()=="" || u.User!=nil || u.Fragment!="" {
        return nil,errors.New("invalid HTTPS resource URL")
    }
    return u,nil
}
func jpGet(ctx context.Context, raw, auth string, limit int64, dest any) error {
    u,e:=jpHTTPS(raw);if e!=nil{return e}
    req,e:=http.NewRequestWithContext(ctx,http.MethodGet,u.String(),nil);if e!=nil{return e}
    if auth!="" {
        if u.Host!="api-launcher-jp.yo-star.com" { return errors.New("refusing auth to non-API origin") }
        req.Header.Set("Authorization",auth)
        req.Header.Set("Content-Type","application/json;charset=UTF-8")
    } else {req.Header.Set("Cache-Control","no-cache")}
    // Never forward the client metadata signature to other origins.
    c:=*client
    c.Timeout=90*time.Second
    c.CheckRedirect=func(req *http.Request, via []*http.Request) error {
        if len(via)>=5 || req.URL.Scheme!="https" || (auth!="" && req.URL.Host!=u.Host) {return errors.New("unexpected metadata redirect")}
        return nil
    }
    res,e:=c.Do(req);if e!=nil{return e};defer res.Body.Close()
    if res.StatusCode!=200{return fmt.Errorf("Yostar metadata HTTP %d (%s)",res.StatusCode,u.Host)}
    b,e:=io.ReadAll(io.LimitReader(res.Body,limit+1));if e!=nil{return e}
    if int64(len(b))>limit{return errors.New("metadata exceeds size limit")}
    return json.Unmarshal(b,dest)
}
func jpRequest(ctx context.Context, path, version string, dest any) error {
    var envelope struct { Code int `json:"code"`; Data json.RawMessage `json:"data"` }
    if e:=jpGet(ctx,jpAPI+path,jpAuth(version,time.Now().Unix()),4<<20,&envelope);e!=nil{return e}
    if envelope.Code!=200{return fmt.Errorf("Yostar API %s returned code %d",strings.Split(path,"?")[0],envelope.Code)}
    return json.Unmarshal(envelope.Data,dest)
}
func jpDecimal(raw json.RawMessage) (uint64,error) {
    s:=string(raw)
    if len(s)>0 && s[0]=='"' {
        if e:=json.Unmarshal(raw,&s);e!=nil{return 0,e}
    }
    n,e:=strconv.ParseUint(s,10,64)
    if e!=nil || strconv.FormatUint(n,10)!=s {return 0,errors.New("noncanonical unsigned decimal")}
    return n,nil
}
func jpRelative(p string) bool {
    if p=="" || strings.ContainsAny(p,"\\:\x00\r\n") || strings.HasPrefix(p,"/") {return false}
    for _, part:=range strings.Split(p,"/") {
        if part=="" || part=="." || part==".." {return false}
    }
    return true
}
func jpParseManifest(raw jpRawManifest, config jpGameConfig, domains jpDomains) (Manifest,error) {
    m:=Manifest{Version:config.Version,VersionName:config.Version,PackageType:2}
    if config.Version=="" || len(raw.File)==0 || len(raw.File)>1000000 {return m,errors.New("missing version or invalid file count")}
    source:=strings.Trim(raw.Source,"/")
    if !jpRelative(source) {return m,errors.New("invalid relative manifest source")}
    // Original worker replaces CDN pathname with source/file path, preserving
    // the selected CDN origin. Query-bearing CDN roots are not yet supported.
    for _,base:=range []string{domains.Primary,domains.Backup} {
        if base=="" {continue}
        u,e:=jpHTTPS(base);if e!=nil{return m,e}
        if u.RawQuery!="" {return m,errors.New("unexpected CDN query")}
        u.Path="/";u.RawPath=""
        if len(m.CDN)==0 || m.CDN[0]!=u.String(){m.CDN=append(m.CDN,u.String())}
    }
    if len(m.CDN)==0{return m,errors.New("no official CDN")}
    seen:=make(map[string]bool,len(raw.File));dirs:=make(map[string]bool)
    for _,f:=range raw.File{
        // Official JP paths start with a single slash; the original Node path.join
        // and CDN path split treat it as game-root-relative. Strip exactly one.
        name:=strings.TrimPrefix(f.Path,"/")
        low:=strings.ToLower(name)
        if !jpRelative(name)||reservedGamePath(name)||low==jpState||strings.HasPrefix(low,jpState+"/")||strings.HasSuffix(low,".part")||seen[low] {return m,fmt.Errorf("unsafe or duplicate manifest path %q",name)}
        if dirs[low]{return m,fmt.Errorf("file/directory conflict %q",name)}
        for p:=filepath.ToSlash(filepath.Dir(low));p!=".";p=filepath.ToSlash(filepath.Dir(p)){
            if seen[p]{return m,fmt.Errorf("file/directory conflict %q",name)}
            dirs[p]=true
        }
        size,e:=jpDecimal(f.Size);if e!=nil||size>=uint64(maxInt64){return m,fmt.Errorf("invalid size for %q",name)}
        hash,e:=jpDecimal(f.Hash);if e!=nil{return m,fmt.Errorf("invalid CRC64 for %q",name)}
        // Use the legacy Entry checksum field with an explicit algorithm tag.
        // url.URL.EscapedPath encodes each path segment without changing slashes.
        remote:=(&url.URL{Path:source+"/"+name}).EscapedPath()
        m.Files=append(m.Files,Entry{Name:name,Remote:remote,MD5:"crc64:"+strconv.FormatUint(hash,10),Size:int64(size)})
        seen[low]=true
    }
    return m,nil
}
func jpDiscover(ctx context.Context, launcherVersion string) (Manifest,jpGameConfig,error) {
    var m Manifest
    var config jpGameConfig
    if e:=jpRequest(ctx,"/api/launcher/game/config",launcherVersion,&config);e!=nil{return m,config,e}
    if config.Version==""||config.Basis==""{return m,config,errors.New("official config contains no released game version or basis")}
    var domains jpDomains
    if e:=jpRequest(ctx,"/api/launcher/advanced/game/download/cdn",launcherVersion,&domains);e!=nil{return m,config,e}
    var mu jpManifestURL
    q:=url.Values{"version":{config.Version},"file_path":{config.Basis}}
    if e:=jpRequest(ctx,"/api/launcher/game/config/json?"+q.Encode(),launcherVersion,&mu);e!=nil{return m,config,e}
    u,e:=jpHTTPS(mu.URL);if e!=nil{return m,config,e}
    query:=u.Query();query.Set("nocache",strconv.FormatInt(time.Now().UnixMilli(),10));u.RawQuery=query.Encode()
    var raw jpRawManifest
    if e=jpGet(ctx,u.String(),"",128<<20,&raw);e!=nil{return m,config,e}
    m,e=jpParseManifest(raw,config,domains)
    return m,config,e
}
func runYostar(args []string) error {
    if len(args)==0{return errors.New("usage: azurpromilia yostar info|install|repair|verify|probe --help")}
    cmd:=args[0]
    if cmd=="help"||cmd=="--help"{fmt.Println("azurpromilia yostar info|install|repair|verify|probe --help");return nil}
    if cmd=="status"||cmd=="resources"{cmd="info"}
    if cmd!="info"&&cmd!="install"&&cmd!="repair"&&cmd!="verify"&&cmd!="probe"{return errors.New("unknown Yostar command")}
    fs:=flag.NewFlagSet("yostar "+cmd,flag.ContinueOnError)
    root:=fs.String("root","AzurPromilia-JP","separate Japanese game directory")
    workers:=fs.Int("workers",4,"parallel downloads")
    version:=fs.String("launcher-version",jpVersion,"official launcher version")
    if e:=fs.Parse(args[1:]);e!=nil{if e==flag.ErrHelp{return nil};return e}
    if *workers<1||*workers>32||*version==""||fs.NArg()!=0{return errors.New("invalid workers, launcher version, or arguments")}
    ctx,cancel:=signal.NotifyContext(context.Background(),os.Interrupt);defer cancel()
    abs,e:=filepath.Abs(*root);if e!=nil{return e}
    if cmd!="info"{
        if st,e:=os.Lstat(abs);e==nil&&st.Mode()&os.ModeSymlink!=0{return errors.New("symlink game root refused")}
        if e=os.MkdirAll(abs,0755);e!=nil{return e}
        for _,p:=range []string{".azurpromilia","_index","_pre_download"}{
            if _,e=os.Lstat(filepath.Join(abs,p));e==nil{return errors.New("SC000 state found; use a separate JP directory")}else if !os.IsNotExist(e){return e}
        }
    }
    state:=filepath.Join(abs,jpState)
    manifestPath,e:=safe(abs,jpState+"/manifest.json");if e!=nil{return e}
    var m Manifest
    if cmd=="verify" {
        b,e:=os.ReadFile(manifestPath);if e!=nil{return e}
        if e=json.Unmarshal(b,&m);e!=nil{return e}
        // Local manifests must pass the same path and algorithm constraints.
        for _,f:=range m.Files{
            if !jpRelative(f.Name)||reservedGamePath(f.Name)||strings.HasPrefix(strings.ToLower(f.Name),jpState)||!strings.HasPrefix(f.MD5,"crc64:")||!validEntryChecksum(f.MD5){return errors.New("invalid saved JP manifest")}
        }
        if len(m.Files)==0{return errors.New("empty saved JP manifest")}
        if e:=runFiles(ctx,abs,m,*workers,true);e!=nil{return e}; fmt.Printf("→ 如需修复，请运行 %s yostar sync\n",cliProgramName()); return nil
    }
    m,config,e:=jpDiscover(ctx,*version);if e!=nil{return e}
    var total int64
    for _,f:=range m.Files{if f.Size>maxInt64-total{return errors.New("total size overflow")};total+=f.Size}
    logf("YOSTAR JP version=%s files=%d bytes=%d CDN=%d launch=%s",config.Version,len(m.Files),total,len(m.CDN),config.LaunchExe)
    if cmd=="info"{
        local := "未安装 / 未记录"
        if b,err:=os.ReadFile(manifestPath);err==nil {var saved Manifest; if json.Unmarshal(b,&saved)==nil && saved.Version!="" {local=saved.Version}}
        printStatusTable(os.Stdout, [][2]string{{"渠道","Yostar JP（实验性；全量安装未验证）"},{"游戏目录",abs},{"本地版本",local},{"官方版本",config.Version},{"资源文件",strconv.Itoa(len(m.Files))},{"资源大小",formatBytes(uint64(total))},{"下载模式","逐文件 / CRC64"},{"CDN",strconv.Itoa(len(m.CDN))},{"启动入口",config.LaunchExe}})
        fmt.Printf("→ %s yostar install / verify / sync；请先使用独立目录测试。\n",cliProgramName())
        return nil
    }
    if cmd=="probe" {
        var chosen *Entry
        for _,f:=range m.Files{if f.Size>0&&(chosen==nil||f.Size<chosen.Size){x:=f;chosen=&x}}
        if chosen==nil||chosen.Size>8<<20{return errors.New("no small nonempty file for probe")}
        m.Files=[]Entry{*chosen}
        logf("JP LIVE PROBE size=%d CRC64=%s",chosen.Size,chosen.MD5)
        return runFiles(ctx,filepath.Join(state,"probe"),m,1,false)
    }
    // Install and repair both converge to the latest official manifest. Do not
    // delete old/unknown files until cross-version behavior is live-validated.
    if e=runFiles(ctx,abs,m,*workers,false);e!=nil{return e}
    if e=os.MkdirAll(state,0755);e!=nil{return e}
    f,e:=os.CreateTemp(state,"manifest-*.tmp");if e!=nil{return e}
    defer os.Remove(f.Name())
    e=json.NewEncoder(f).Encode(m)
    if e==nil{e=f.Sync()}
    closeErr:=f.Close();if e!=nil{return e};if closeErr!=nil{return closeErr}
    if e=os.Rename(f.Name(),manifestPath);e!=nil{return e}
    logf("YOSTAR JP COMPLETE %s",config.Version)
    fmt.Printf("→ 请运行 %s yostar verify；日服全量安装和启动尚未经实机验收。\n",cliProgramName())
    return nil
}
