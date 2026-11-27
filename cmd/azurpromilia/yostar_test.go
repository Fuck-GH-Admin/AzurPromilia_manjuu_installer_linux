package main

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "sync/atomic"
    "testing"
)

func TestYostarCRC(t *testing.T) {
    if !verifyEntryHash(bytes.NewBufferString("123456789"),"crc64:11051210869376104954") {t.Fatal("CRC64/XZ check vector mismatch")}
    if !verifyEntryHash(bytes.NewReader(nil),"crc64:0") {t.Fatal("empty CRC mismatch")}
    if verifyEntryHash(bytes.NewBufferString("x"),"crc64:0") || validEntryChecksum("crc64:18446744073709551616") {t.Fatal("invalid CRC accepted")}
}
func TestYostarManifest(t *testing.T) {
    raw:=jpRawManifest{Source:"/game/v1/",File:[]jpRawFile{{Path:"/abc/日本 語#1.bin",Size:json.RawMessage(`"9"`),Hash:json.RawMessage(`"11051210869376104954"`)}}}
    c:=jpGameConfig{Version:"1.0"}
    d:=jpDomains{Primary:"https://cdn.example.com/ignored"}
    m,e:=jpParseManifest(raw,c,d)
    if e!=nil||len(m.Files)!=1||m.Files[0].MD5!="crc64:11051210869376104954"||m.CDN[0]!="https://cdn.example.com/"||m.Files[0].Remote!="game/v1/abc/%E6%97%A5%E6%9C%AC%20%E8%AA%9E%231.bin"{t.Fatalf("manifest: %+v %v",m,e)}
    for _,p:=range []string{"../bad","abc/../../bad","//root","/../root","abc\\bad",".azurpromilia-yostar/manifest.json","_index/version","bad.part"}{
        raw.File[0].Path=p
        if _,e=jpParseManifest(raw,c,d);e==nil{t.Fatalf("unsafe path accepted: %q",p)}
    }
}
func TestYostarWorkerResume(t *testing.T){
    root:=t.TempDir()
    data:=[]byte("123456789")
    srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        if r.Header.Get("Range")!="bytes=3-"{t.Errorf("wrong range %s",r.Header.Get("Range"))}
        w.Header().Set("Content-Range","bytes 3-8/9")
        w.WriteHeader(http.StatusPartialContent)
        _,_=w.Write(data[3:])
    }));defer srv.Close()
    if e:=os.WriteFile(filepath.Join(root,"sample.part"),data[:3],0600);e!=nil{t.Fatal(e)}
    entry:=Entry{Name:"sample",Remote:srv.URL,MD5:"crc64:11051210869376104954",Size:9}
    var n atomic.Int64
    if e:=download(context.Background(),root,entry,nil,&n);e!=nil{t.Fatal(e)}
    if !verify(filepath.Join(root,"sample"),entry)||n.Load()!=6{t.Fatal("resume failed")}
}
