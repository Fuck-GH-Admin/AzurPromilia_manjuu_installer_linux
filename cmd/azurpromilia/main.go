package main

import (
 "archive/zip"
 "bufio"
 "bytes"
 "context"
 "crypto/aes"
 "crypto/cipher"
 "crypto/md5"
 "hash/crc32"
 "encoding/base64"
 "encoding/hex"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "os/signal"
 "path/filepath"
 "regexp"
 "strconv"
 "strings"
 "sync"
 "sync/atomic"
 "time"
)

type Package struct {
 Type int `json:"packageType"`
 URL string `json:"downloadUrl"`
 MD5 string `json:"fileMd5"`
 Size int64 `json:"fileSize"`
 CDN []string `json:"cdnBaseUrls"`
}
type ExcludedFile struct {
 FilePath string `json:"filePath"`
 FileType int `json:"fileType"`
}
type Release struct {
 Version json.RawMessage `json:"versionCode"`
 Name string `json:"versionName"`
 GameName string `json:"gameName"`
 GameFolderName string `json:"gameFolderName"`
 LaunchEntry string `json:"launchEntry"`
 PackageSize int64 `json:"packageSize"`
 GameExcludedFileList []ExcludedFile `json:"gameExcludedFileList"`
 CDN []string `json:"domainList"`
 Packages []Package `json:"gamePackageList"`
}
type Response struct {
 Code int `json:"code"`
 Data struct {
  DownloadMode int `json:"downloadMode"`
  Released Release `json:"released"`
  Preload Release `json:"preload"`
  GameConfig json.RawMessage `json:"gameConfig"`
  LaunchInfo json.RawMessage `json:"launchInfo"`
 } `json:"data"`
}
type Entry struct { Name, Remote, MD5 string; Size int64 }
type ExtractedEntry struct {
 Name string `json:"name"`
 Size int64 `json:"size"`
 CRC32 uint32 `json:"crc32"`
}
type Manifest struct {
 Version string
 VersionName string
 PackageType int
 Files []Entry
 Archive *Entry
 CDN []string
 Directories []string
 Excluded []ExcludedFile
 Extracted []ExtractedEntry
}
var client = &http.Client{Transport:&http.Transport{Proxy:http.ProxyFromEnvironment,MaxIdleConns:32,MaxIdleConnsPerHost:16,ResponseHeaderTimeout:30*time.Second,IdleConnTimeout:30*time.Second,DisableCompression:true}}
var output sync.Mutex
func logf(f string,a ...any){output.Lock();defer output.Unlock();fmt.Fprintf(os.Stderr,f+"\n",a...)}
func validMD5(s string)bool{b,e:=hex.DecodeString(s);return e==nil&&len(b)==16}
func safe(root,name string)(string,error){
 name=strings.ReplaceAll(name,"\\","/")
 if name==""||strings.HasPrefix(name,"/")||strings.Contains(name,":") {return "",fmt.Errorf("unsafe path %q",name)}
 for _,p:=range strings.Split(name,"/"){if p==".."||p=="."||p==""{return "",fmt.Errorf("unsafe path %q",name)}}
 p:=filepath.Join(root,filepath.FromSlash(name))
 for x:=p;x!=root&&x!=filepath.Dir(x);x=filepath.Dir(x){if st,e:=os.Lstat(x);e==nil&&st.Mode()&os.ModeSymlink!=0{return "",fmt.Errorf("symlink refused: %s",x)}}
 return p,nil
}
func verify(p string,e Entry)bool{
 f,err:=os.Open(p);if err!=nil{return false};defer f.Close()
 st,err:=f.Stat();if err!=nil||!st.Mode().IsRegular()||st.Size()!=e.Size{return false}
 h:=md5.New();_,err=io.Copy(h,f);return err==nil&&strings.EqualFold(hex.EncodeToString(h.Sum(nil)),e.MD5)
}
func getURLs(cdn []string,remote string)([]string,error){
 u,err:=url.Parse(remote);if err!=nil{return nil,err}
 if u.IsAbs(){if u.Scheme!="https"&&u.Scheme!="http"{return nil,errors.New("unsupported scheme")};return []string{remote},nil}
 var out []string
 for _,base:=range cdn{v:=strings.TrimRight(base,"/")+"/"+strings.TrimLeft(remote,"/");u,e:=url.Parse(v);if e!=nil||u.Host==""||(u.Scheme!="https"&&u.Scheme!="http"){return nil,fmt.Errorf("invalid CDN %q",base)};out=append(out,v)}
 if len(out)==0{return nil,errors.New("no CDN")};return out,nil
}
type counter struct{ n *atomic.Int64 }
func(c counter)Write(p []byte)(int,error){c.n.Add(int64(len(p)));return len(p),nil}
func download(ctx context.Context,root string,e Entry,cdn []string,network *atomic.Int64)error{
 p,err:=safe(root,e.Name);if err!=nil{return err};if !validMD5(e.MD5)||e.Size<0{return errors.New("invalid file checksum or size")}
 if verify(p,e){return nil}
 if err=os.MkdirAll(filepath.Dir(p),0755);err!=nil{return err}
 part:=p+".part";if st,err:=os.Lstat(part);err==nil&&!st.Mode().IsRegular(){return fmt.Errorf("unsafe partial file %s",part)}
 urls,err:=getURLs(cdn,e.Remote);if err!=nil{return err}
 var last error
 for attempt:=0;attempt<max(6,len(urls)*3);attempt++{
  if ctx.Err()!=nil{return ctx.Err()}
  if verify(part,e){return os.Rename(part,p)}
  off:=int64(0);if st,er:=os.Stat(part);er==nil{off=st.Size();if off>=e.Size{if er=os.Remove(part);er!=nil{return er};off=0}}
  if e.Size==0{if err=os.WriteFile(part,nil,0644);err!=nil{return err};if !verify(part,e){return errors.New("empty file checksum mismatch")};return os.Rename(part,p)}
  req,er:=http.NewRequestWithContext(ctx,"GET",urls[(attempt/3)%len(urls)],nil);if er!=nil{return er}
  if off>0{req.Header.Set("Range",fmt.Sprintf("bytes=%d-",off))}
  res,er:=client.Do(req)
  if er==nil{
   if res.StatusCode==http.StatusPartialContent{
    want:=fmt.Sprintf("bytes %d-%d/%d",off,e.Size-1,e.Size)
    if res.Header.Get("Content-Range")!=want{er=fmt.Errorf("invalid Content-Range: %s",res.Header.Get("Content-Range"))}
   }else if res.StatusCode==http.StatusOK{off=0}else{er=fmt.Errorf("HTTP %d",res.StatusCode)}
   if er==nil{
    mode:=os.O_CREATE|os.O_WRONLY;if off>0{mode|=os.O_APPEND}else{mode|=os.O_TRUNC}
    f,fe:=os.OpenFile(part,mode,0644);if fe!=nil{res.Body.Close();return fe}
    n,ce:=io.Copy(io.MultiWriter(f,counter{network}),io.LimitReader(res.Body,e.Size-off+1));se:=f.Close()
    if ce!=nil{er=ce}else if se!=nil{er=se}else if n!=e.Size-off{er=fmt.Errorf("incomplete response: %d/%d bytes",n,e.Size-off)}
   };res.Body.Close()
  }
  if er==nil&&verify(part,e){return os.Rename(part,p)}
  if er==nil{er=errors.New("MD5 mismatch");_ = os.Remove(part)}
  last=er;logf("RETRY %s (%d): %v",e.Name,attempt+1,last)
  select{case <-ctx.Done():return ctx.Err();case <-time.After(time.Duration(attempt+1)*time.Second):}
 }
 return fmt.Errorf("%s: %w",e.Name,last)
}
func decrypt(data []byte,key,iv string)([]byte,error){
 b,err:=base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)));if err!=nil{return nil,err}
 c,err:=aes.NewCipher([]byte(key));if err!=nil{return nil,err}
 if len(iv)!=16||len(b)==0||len(b)%16!=0{return nil,errors.New("invalid CBC length")}
 cipher.NewCBCDecrypter(c,[]byte(iv)).CryptBlocks(b,b)
 n:=int(b[len(b)-1]);if n<1||n>16||n>len(b){return nil,errors.New("invalid PKCS7 padding")}
 for _,v:=range b[len(b)-n:]{if int(v)!=n{return nil,errors.New("invalid PKCS7 padding")}}
 return b[:len(b)-n],nil
}
// Locate the installer embedded ZIP by its central directory, then read the
// bundled Electron application locally. No vendor keys are committed.
func installerKeys(path string)(string,string,error){
 b,err:=os.ReadFile(path);if err!=nil{return "","",err}
 sig:=[]byte{'P','K',5,6};end:=bytes.LastIndex(b,sig)
 for end>=0{
  if end+22<=len(b){
   z,e:=zip.NewReader(bytes.NewReader(b[:end+22]),int64(end+22))
   if e==nil{for _,f:=range z.File{if f.Name!="resources/app.asar"{continue};r,e:=f.Open();if e!=nil{return "","",e};data,e:=io.ReadAll(r);r.Close();if e!=nil{return "","",e}
    re:=regexp.MustCompile("\"HM\":\"([^\"]+)\"\\s*,\\s*\"rs\":\"([^\"]+)\"");m:=re.FindSubmatch(data);if len(m)==3{return string(m[1]),string(m[2]),nil}
   }}
  }
  end=bytes.LastIndex(b[:end],sig)
 }
 return "","",errors.New("installer index configuration not found")
}
func parseIndex(raw []byte,slice bool)(Manifest,error){
 var m Manifest;seen:=map[string]bool{}
 records := 0
 s:=bufio.NewScanner(bytes.NewReader(raw));s.Buffer(make([]byte,65536),4<<20)
 for s.Scan(){
  line:=strings.TrimSuffix(strings.TrimSpace(s.Text()),",");if line==""{continue}
  a:=strings.Split(line,",")
  records++
  // A per-file index can start with counts for all records, files, and directories.
  if !slice && records == 1 && len(a) == 3 {
   for _, field := range a {
    if _, err := strconv.ParseUint(field, 10, 64); err != nil {
     return m, errors.New("invalid per-file index counts")
    }
   }
   continue
  }
  if slice&&len(a)==3{n,e:=strconv.ParseInt(a[1],10,64);if e!=nil||n<0||!validMD5(a[2])||m.Archive!=nil{return m,errors.New("invalid ZIP record")};m.Archive=&Entry{Name:"game.zip",Size:n,MD5:a[2]};continue}
  if !slice&&len(a)==4{m.Directories=append(m.Directories,strings.ReplaceAll(a[3],"\\","/"));continue}
  if len(a)!=5{return m,fmt.Errorf("unknown index row (%d columns)",len(a))}
  name,size,hash:=a[3],a[1],a[2];if slice{name,size,hash=a[3],a[2],a[3]}
  name=strings.ReplaceAll(name,"\\","/");n,e:=strconv.ParseInt(size,10,64)
  if e!=nil||n<0||!validMD5(hash)||seen[strings.ToLower(name)]{return m,fmt.Errorf("invalid or duplicate record %q",name)}
  if _,e=safe("/tmp",name);e!=nil{return m,e}
  if strings.HasPrefix(name,".azurpromilia/")||strings.HasSuffix(name,".part"){return m,errors.New("reserved file path")}
  seen[strings.ToLower(name)]=true;m.Files=append(m.Files,Entry{name,a[4],hash,n})
 }
 if err:=s.Err();err!=nil{return m,err}
 if len(m.Files)==0||slice&&m.Archive==nil{return m,errors.New("empty index or missing ZIP record")};return m,nil
}
func runFiles(ctx context.Context,root string,m Manifest,workers int,onlyVerify bool)error{
 ctx,cancel:=context.WithCancel(ctx);defer cancel()
 var net,done,checked,fail atomic.Int64;var total int64
 for _,e:=range m.Files{total+=e.Size}
 jobs:=make(chan Entry);var wg sync.WaitGroup;var first error;var mu sync.Mutex
 stop:=make(chan struct{});defer close(stop)
 go func(){t:=time.NewTicker(time.Second);defer t.Stop();var last int64;for{select{case <-stop:return;case <-t.C:n:=net.Load();logf("PROGRESS %d/%d files | %.2f/%.2f GiB verified | %.2f MiB/s | failed %d",checked.Load(),len(m.Files),float64(done.Load())/(1<<30),float64(total)/(1<<30),float64(n-last)/(1<<20),fail.Load());last=n}}}()
 for i:=0;i<workers;i++{wg.Add(1);go func(){defer wg.Done();for e:=range jobs{
  p,err:=safe(root,e.Name)
  if err==nil{if onlyVerify{if !verify(p,e){err=errors.New("missing or invalid")}}else{logf("FILE %s (%d bytes)",e.Name,e.Size);err=download(ctx,root,e,m.CDN,&net)}}
  checked.Add(1);if err!=nil{fail.Add(1);logf("FAIL %s: %v",e.Name,err);mu.Lock();if first==nil{first=fmt.Errorf("%s: %w",e.Name,err)};mu.Unlock()}else{done.Add(e.Size)}
 }}()}
 send:for _,e:=range m.Files{select{case jobs<-e:case <-ctx.Done():break send}}
 close(jobs);wg.Wait();logf("RESULT %d/%d verified, %d failed; %d bytes transferred",checked.Load()-fail.Load(),len(m.Files),fail.Load(),net.Load())
 if ctx.Err()!=nil{return ctx.Err()};return first
}
func writeJSON(path string,v any)error{b,e:=json.MarshalIndent(v,"","  ");if e!=nil{return e};if e=os.MkdirAll(filepath.Dir(path),0755);e!=nil{return e};return os.WriteFile(path,b,0644)}
func merge(root string,m Manifest,out string)error{
 if verify(out,*m.Archive){return nil}
 if e:=os.MkdirAll(filepath.Dir(out),0755);e!=nil{return e}
 f,e:=os.Create(out+".part");if e!=nil{return e}
 for _,v:=range m.Files{p,e:=safe(root,v.Name);if e!=nil{f.Close();return e};r,e:=os.Open(p);if e!=nil{f.Close();return e};_,e=io.Copy(f,r);r.Close();if e!=nil{f.Close();return e}}
 if e=f.Close();e!=nil{return e};if !verify(out+".part",*m.Archive){return errors.New("merged ZIP checksum mismatch")};return os.Rename(out+".part",out)
}
func extractZIP(path,root string)error{
 z,e:=zip.OpenReader(path);if e!=nil{return e};defer z.Close()
 for _,f:=range z.File{
  name:=strings.TrimSuffix(strings.ReplaceAll(f.Name,"\\","/"),"/")
  if strings.HasPrefix(name,".azurpromilia")||f.Mode()&os.ModeSymlink!=0{return fmt.Errorf("unsafe ZIP entry %s",name)}
  p,e:=safe(root,name);if e!=nil{return e}
  if f.FileInfo().IsDir(){if e=os.MkdirAll(p,0755);e!=nil{return e};continue}
  if e=os.MkdirAll(filepath.Dir(p),0755);e!=nil{return e}
  r,e:=f.Open();if e!=nil{return e};tmp,e:=os.CreateTemp(filepath.Dir(p),".ap-extract-");if e!=nil{r.Close();return e}
  _,ce:=io.Copy(tmp,r);r.Close();se:=tmp.Close();if ce!=nil||se!=nil{os.Remove(tmp.Name());return errors.Join(ce,se)}
  if e=os.Rename(tmp.Name(),p);e!=nil{os.Remove(tmp.Name());return e}
 };return nil
}
func verifyExtracted(archive,root string)error{
 z,e:=zip.OpenReader(archive);if e!=nil{return e};defer z.Close()
 var failed int
 for _,v:=range z.File{
  if v.FileInfo().IsDir(){continue}
  p,e:=safe(root,strings.ReplaceAll(v.Name,"\\","/"));if e!=nil{return e}
  f,e:=os.Open(p);if e!=nil{failed++;logf("FAIL extracted %s: %v",v.Name,e);continue}
  h:=crc32.NewIEEE();n,er:=io.Copy(h,f);f.Close()
  if er!=nil||uint64(n)!=v.UncompressedSize64||h.Sum32()!=v.CRC32{failed++;logf("FAIL extracted %s",v.Name)}
 }
 if failed>0{return fmt.Errorf("%d extracted files invalid or missing",failed)};logf("Extracted ZIP files verified");return nil
}
func main(){if err:=run();err!=nil{logf("ERROR: %v",err);os.Exit(1)}}
func run()error{
 if len(os.Args)<2{return errors.New("usage: azurpromilia download|verify --help")}
 cmd:=os.Args[1];if cmd=="help"||cmd=="--help"{fmt.Println("azurpromilia install|update|repair|preload|info|download|verify --help");return nil};if cmd=="install"||cmd=="update"||cmd=="repair"||cmd=="preload"||cmd=="info"{return runWorkflow(cmd,os.Args[2:])}
 fs:=flag.NewFlagSet(cmd,flag.ContinueOnError);root:=fs.String("root","AzurPromilia","game directory");pkgPath:=fs.String("packageconfig","","official packageconfig response JSON");exe:=fs.String("installer","","original installer EXE");target:=fs.String("target","released","released or preload");workers:=fs.Int("workers",4,"parallel downloads");slice:=fs.Bool("slice",false,"select slice package");noExtract:=fs.Bool("no-extract",false,"do not extract ZIP");probe:=fs.Bool("probe",false,"only fetch official index and one smallest nonempty resource; do not install");versionFlag:=fs.String("version","","preload version for verification");mode:=fs.String("mode","auto","auto, slice or reality");if e:=fs.Parse(os.Args[2:]);e!=nil{if e==flag.ErrHelp{return nil};return e}
 if *workers<1||*workers>32{return errors.New("workers must be 1..32")}
 abs,e:=filepath.Abs(*root);if e!=nil{return e};if e=os.MkdirAll(abs,0755);e!=nil{return e};abs,e=filepath.EvalSymlinks(abs);if e!=nil{return e}
 ctx,cancel:=signal.NotifyContext(context.Background(),os.Interrupt);defer cancel()
 state:=filepath.Join(abs,".azurpromilia");manifestPath:=filepath.Join(state,"manifest.json");var m Manifest
 if cmd=="verify"{if *target=="preload"{if *versionFlag==""||!regexp.MustCompile("^[a-zA-Z0-9_.-]+$").MatchString(*versionFlag){return errors.New("verify preload requires --version")};manifestPath=filepath.Join(state,"preload-"+*versionFlag+".json")}else if *target!="released"{return errors.New("invalid target")};b,e:=os.ReadFile(manifestPath);if e!=nil{return e};if e=json.Unmarshal(b,&m);e!=nil{return e};work:=abs;if *target=="preload"{work=filepath.Join(abs,"_pre_download",m.Version)};if len(m.Extracted)>0{if e=verifyExtractedEntries(work,m.Extracted);e!=nil{return e};if len(m.Files)==0{return nil}};if m.Archive!=nil{archive:=filepath.Join(state,"archives",m.Version+".zip");if _,statErr:=os.Stat(archive);statErr==nil{if !verify(archive,*m.Archive){return errors.New("merged archive missing or invalid")};if !*noExtract{if e=verifyExtracted(archive,work);e!=nil{return e}}}else if len(m.Extracted)==0{return errors.New("merged archive missing or invalid")};work=filepath.Join(state,"slices",m.Version)};return runFiles(ctx,work,m,*workers,true)}
 if cmd!="download"{return errors.New("unknown command")};if *pkgPath==""||*exe==""{return errors.New("--packageconfig and --installer required")}
 key,iv,e:=installerKeys(*exe);if e!=nil{return e};raw,e:=os.ReadFile(*pkgPath);if e!=nil{return e}
 var response Response;if e=json.Unmarshal(raw,&response);e!=nil{return e};if response.Code!=0{return fmt.Errorf("official API error %d",response.Code)}
 release:=response.Data.Released;if *target=="preload"{release=response.Data.Preload}else if *target!="released"{return errors.New("invalid target")}
 version:=strings.Trim(string(release.Version),"\"");if version==""||version=="null"||!regexp.MustCompile("^[a-zA-Z0-9_.-]+$").MatchString(version)||version=="."||version==".."{return errors.New("invalid/missing version")}
 typ,e:=selectPackageType(response.Data.DownloadMode,*mode,release.Packages);if e!=nil{return e};if *slice{typ=1};if *target=="preload"{typ=2};var pkg *Package
 for _,p:=range release.Packages{if p.Type==typ{v:=p;pkg=&v;break}}
 if pkg==nil{return fmt.Errorf("package type %d unavailable; inspect official response",typ)}
 cdn:=pkg.CDN;if len(release.CDN)>0{cdn=release.CDN}
 logf("VERSION %s (%s) package type %d; CDN count %d",version,release.Name,typ,len(cdn))
 indexRoot:=filepath.Join(state,"index");var network atomic.Int64
 index:=Entry{version+"-"+strconv.Itoa(typ),pkg.URL,pkg.MD5,pkg.Size}
 if e=download(ctx,indexRoot,index,cdn,&network);e!=nil{return e};raw,e=os.ReadFile(filepath.Join(indexRoot,index.Name));if e!=nil{return e}
 plain,e:=decrypt(raw,key,iv);if e!=nil{return e};m,e=parseIndex(plain,*slice);if e!=nil{return e};m.Version=version;m.CDN=cdn
 if *probe{var chosen *Entry;for _,f:=range m.Files{if f.Size>0&&(chosen==nil||f.Size<chosen.Size){v:=f;chosen=&v}};if chosen==nil{return errors.New("no nonempty resources")};if chosen.Size>64<<20{return errors.New("smallest resource exceeds 64 MiB probe limit")};logf("LIVE PROBE: %s, %d bytes",chosen.Name,chosen.Size);if e=download(ctx,filepath.Join(state,"probe"),*chosen,cdn,&network);e!=nil{return e};logf("LIVE PROBE OK: official index and resource verified");return nil}
 work:=abs;if *target=="preload"{work=filepath.Join(abs,"_pre_download",version);manifestPath=filepath.Join(state,"preload-"+version+".json")};if *slice{work=filepath.Join(state,"slices",version)}
 if e=runFiles(ctx,work,m,*workers,false);e!=nil{return e}
 if m.Archive!=nil{out:=filepath.Join(state,"archives",version+".zip");if e=merge(work,m,out);e!=nil{return e};logf("ZIP verified: %s",out);if !*noExtract{dest:=abs;if *target=="preload"{dest=filepath.Join(abs,"_pre_download",version)};if e=extractZIP(out,dest);e!=nil{return e}}}else{for _,d:=range m.Directories{p,e:=safe(work,d);if e!=nil{return e};if e=os.MkdirAll(p,0755);e!=nil{return e}}}
 if e=writeJSON(manifestPath,m);e!=nil{return e};logf("COMPLETE %s | %s",version,manifestPath);return nil
}
