package main

import (
 "archive/zip"
 "bufio"
 "bytes"
 "context"
 "crypto/aes"
 "crypto/cipher"
 "crypto/md5"
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
 if st,e:=os.Lstat(root);e==nil&&st.Mode()&os.ModeSymlink!=0{return "",fmt.Errorf("symlink refused for root: %s",root)}
 name=strings.ReplaceAll(name,"\\","/")
 if name==""||strings.HasPrefix(name,"/")||strings.Contains(name,":") {return "",fmt.Errorf("unsafe path %q",name)}
 for _,p:=range strings.Split(name,"/"){if p==".."||p=="."||p==""{return "",fmt.Errorf("unsafe path %q",name)}}
 p:=filepath.Join(root,filepath.FromSlash(name))
 for x:=p;x!=root&&x!=filepath.Dir(x);x=filepath.Dir(x){if st,e:=os.Lstat(x);e==nil&&st.Mode()&os.ModeSymlink!=0{return "",fmt.Errorf("symlink refused: %s",x)}}
 return p,nil
}
func reservedGamePath(name string) bool {
 name = strings.ToLower(strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/"))
 for _, prefix := range []string{".azurpromilia", "_index", "_pre_download"} {
  if name == prefix || strings.HasPrefix(name, prefix+"/") { return true }
 }
 return false
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
func download(ctx context.Context, root string, e Entry, cdn []string, network *atomic.Int64) error {
	return downloadFile(ctx, root, e, cdn, network, true)
}

func downloadAfterPreflight(ctx context.Context, root string, e Entry, cdn []string, network *atomic.Int64) error {
	return downloadFile(ctx, root, e, cdn, network, false)
}

func downloadFile(ctx context.Context,root string,e Entry,cdn []string,network *atomic.Int64,checkExisting bool)error{
 p,err:=safe(root,e.Name);if err!=nil{return err};if !validMD5(e.MD5)||e.Size<0{return errors.New("invalid file checksum or size")}
 if st, statErr := os.Lstat(p); statErr == nil && !st.Mode().IsRegular() { return fmt.Errorf("download destination is not a regular file: %s", p) } else if statErr != nil && !os.IsNotExist(statErr) { return statErr }; if checkExisting && verify(p,e){return nil}
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
 var expectedTotal, expectedFiles, expectedDirs uint64
 hasCounts := false
 s:=bufio.NewScanner(bytes.NewReader(raw));s.Buffer(make([]byte,65536),4<<20)
 for s.Scan(){
  line:=strings.TrimSuffix(strings.TrimSpace(s.Text()),",");if line==""{continue}
  a:=strings.Split(line,",")
  records++
  // A per-file index can start with counts for all records, files, and directories.
  if !slice && records == 1 && len(a) == 3 {
   counts := []*uint64{&expectedTotal, &expectedFiles, &expectedDirs}
   for i, field := range a {
    n, err := strconv.ParseUint(field, 10, 64)
    if err != nil { return m, errors.New("invalid per-file index counts") }
    *counts[i] = n
   }
   hasCounts = true
   continue
  }
  if slice&&len(a)==3{n,e:=strconv.ParseInt(a[1],10,64);if e!=nil||n<0||!validMD5(a[2])||m.Archive!=nil{return m,errors.New("invalid ZIP record")};m.Archive=&Entry{Name:"game.zip",Size:n,MD5:a[2]};continue}
  if !slice&&len(a)==4{dir:=strings.ReplaceAll(a[3],"\\","/");if reservedGamePath(dir){return m,fmt.Errorf("reserved directory path %q",dir)};m.Directories=append(m.Directories,dir);continue}
  if len(a)!=5{return m,fmt.Errorf("unknown index row (%d columns)",len(a))}
  name,size,hash:=a[3],a[1],a[2];if slice{name,size,hash=a[3],a[2],a[3]}
  name=strings.ReplaceAll(name,"\\","/");n,e:=strconv.ParseInt(size,10,64)
  if e!=nil||n<0||!validMD5(hash)||seen[strings.ToLower(name)]{return m,fmt.Errorf("invalid or duplicate record %q",name)}
  if _,e=safe("/tmp",name);e!=nil{return m,e}
  if reservedGamePath(name)||strings.HasSuffix(strings.ToLower(name),".part"){return m,errors.New("reserved file path")}
  seen[strings.ToLower(name)]=true;m.Files=append(m.Files,Entry{name,a[4],hash,n})
 }
 if err:=s.Err();err!=nil{return m,err}
 if hasCounts && (uint64(len(m.Files)) != expectedFiles || uint64(len(m.Directories)) != expectedDirs || uint64(len(m.Files)+len(m.Directories)) != expectedTotal) { return m, fmt.Errorf("per-file index count mismatch: header total=%d files=%d dirs=%d, parsed total=%d files=%d dirs=%d", expectedTotal, expectedFiles, expectedDirs, len(m.Files)+len(m.Directories), len(m.Files), len(m.Directories)) }
 if len(m.Files)==0||slice&&m.Archive==nil{return m,errors.New("empty index or missing ZIP record")};return m,nil
}
type fileWorkerStats struct {
	Completed int64
	Failed int64
	Cancelled int64
	First error
}

// runFileWorkers owns the worker lifecycle. Download workflows stop dispatching
// immediately after the first fatal file error and cancel in-flight HTTP requests.
// Verification deliberately keeps scanning so users get a complete failure list.
func runFileWorkers(ctx context.Context, files []Entry, workers int, stopOnError bool, process func(context.Context, Entry) error) fileWorkerStats {
	var stats fileWorkerStats
	if workers < 1 {
		stats.First = errors.New("workers must be at least 1")
		return stats
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan Entry)
	var wg sync.WaitGroup
	var completed, failed, cancelled atomic.Int64
	var first error
	var firstMu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				if workCtx.Err() != nil {
					cancelled.Add(1)
					continue
				}
				err := process(workCtx, entry)
				if err == nil {
					completed.Add(1)
					continue
				}
				if workCtx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
					cancelled.Add(1)
					continue
				}
				failed.Add(1)
				firstMu.Lock()
				if first == nil {
					first = err
					if stopOnError {
						cancel()
					}
				}
				firstMu.Unlock()
			}
		}()
	}

send:
	for _, entry := range files {
		select {
		case <-workCtx.Done():
			break send
		case jobs <- entry:
		}
	}
	close(jobs)
	wg.Wait()

	stats.Completed = completed.Load()
	stats.Failed = failed.Load()
	stats.Cancelled = cancelled.Load()
	stats.First = first
	if stats.First == nil && ctx.Err() != nil {
		stats.First = ctx.Err()
	}
	return stats
}

func runFiles(ctx context.Context, root string, m Manifest, workers int, onlyVerify bool) error {
	var network, done, verified atomic.Int64
	var total int64
	for _, entry := range m.Files {
		if entry.Size < 0 || entry.Size > maxInt64-total {
			return fmt.Errorf("invalid or overflowing manifest size at %s", entry.Name)
		}
		total += entry.Size
	}

	valid := map[string]bool(nil)
	if !onlyVerify {
		var required int64
		var err error
		valid, required, err = preflightFiles(root, m.Files)
		if err != nil {
			return err
		}
		logf("SPACE preflight: %s additional space required for downloads", formatBytes(uint64(required)))
	}

	stop := make(chan struct{})
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var last int64
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				n := network.Load()
				logf("PROGRESS %d/%d verified | %.2f/%.2f GiB | %.2f MiB/s",
					verified.Load(), len(m.Files), float64(done.Load())/(1<<30),
					float64(total)/(1<<30), float64(n-last)/(1<<20))
				last = n
			}
		}
	}()

	process := func(workCtx context.Context, entry Entry) error {
		p, err := safe(root, entry.Name)
		if err != nil {
			return err
		}
		if onlyVerify {
			if !verify(p, entry) {
				return fmt.Errorf("%s: missing or invalid", entry.Name)
			}
		} else if !valid[entry.Name] {
			logf("FILE %s (%d bytes)", entry.Name, entry.Size)
			if err = downloadAfterPreflight(workCtx, root, entry, m.CDN, &network); err != nil {
				return err
			}
		}
		done.Add(entry.Size)
		verified.Add(1)
		return nil
	}

	stats := runFileWorkers(ctx, m.Files, workers, !onlyVerify, process)
	close(stop)
	<-progressDone
	logf("RESULT %d/%d verified, %d failed, %d cancelled; %d bytes transferred",
		stats.Completed, len(m.Files), stats.Failed, stats.Cancelled, network.Load())
	if stats.First != nil {
		return stats.First
	}
	return nil
}

func writeJSON(path string,v any)error{b,e:=json.MarshalIndent(v,"","  ");if e!=nil{return e};if e=os.MkdirAll(filepath.Dir(path),0755);e!=nil{return e};return os.WriteFile(path,b,0644)}
func merge(root string, m Manifest, out string) error {
	if m.Archive == nil {
		return errors.New("missing ZIP archive metadata")
	}
	if verify(out, *m.Archive) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	if err := requireFreeSpace(filepath.Dir(out), m.Archive.Size, "merge slice archive"); err != nil {
		return err
	}
	f, err := os.Create(out + ".part")
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(out + ".part")
		}
	}()
	for _, entry := range m.Files {
		p, err := safe(root, entry.Name)
		if err != nil {
			return err
		}
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, r)
		closeErr := r.Close()
		if copyErr != nil || closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if !verify(out+".part", *m.Archive) {
		return errors.New("merged ZIP checksum mismatch")
	}
	if err = os.Rename(out+".part", out); err != nil {
		return err
	}
	ok = true
	return nil
}

func extractZIP(path, root string) error {
	entries, err := zipExtractedEntries(path)
	if err != nil {
		return err
	}
	if st, statErr := os.Lstat(root); statErr == nil && st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink refused for extraction root: %s", root)
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return err
	}
	required, err := requiredExtractionBytes(root, entries)
	if err != nil {
		return err
	}
	if err = requireFreeSpace(root, required, "ZIP extraction"); err != nil {
		return err
	}
	logf("SPACE preflight: %s additional space required for ZIP extraction", formatBytes(uint64(required)))

	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, f := range z.File {
		name := strings.TrimSuffix(strings.ReplaceAll(f.Name, "\\", "/"), "/")
		p, err := safe(root, name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			dirMode := f.Mode().Perm()
			if dirMode == 0 {
				dirMode = 0755
			}
			if err = os.MkdirAll(p, dirMode); err != nil {
				return err
			}
			continue
		}
		entry := ExtractedEntry{Name: name, Size: int64(f.UncompressedSize64), CRC32: f.CRC32}
		if extractedEntryValid(root, entry) {
			continue
		}
		if err = os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(p), ".ap-extract-*")
		if err != nil {
			r.Close()
			return err
		}
		tmpName := tmp.Name()
		mode := f.Mode().Perm()
		if mode == 0 {
			mode = 0644
		}
		copyErr := tmp.Chmod(mode)
		var copied int64
		if copyErr == nil {
			copied, copyErr = io.Copy(tmp, io.LimitReader(r, entry.Size+1))
		}
		readCloseErr := r.Close()
		if copyErr == nil && copied != entry.Size {
			copyErr = fmt.Errorf("ZIP entry size mismatch for %s: got %d, want %d", name, copied, entry.Size)
		}
		if readCloseErr != nil {
			copyErr = errors.Join(copyErr, readCloseErr)
		}
		if copyErr == nil {
			copyErr = tmp.Sync()
		}
		closeErr := tmp.Close()
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(tmpName)
			return errors.Join(copyErr, closeErr)
		}
		if err = os.Rename(tmpName, p); err != nil {
			_ = os.Remove(tmpName)
			return err
		}
	}
	return nil
}

func verifyExtracted(archive, root string) error {
	entries, err := zipExtractedEntries(archive)
	if err != nil {
		return err
	}
	return verifyExtractedEntries(root, entries)
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
