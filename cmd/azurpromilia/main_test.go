package main
import("context";"crypto/md5";"fmt";"net/http";"net/http/httptest";"os";"path/filepath";"sync/atomic";"testing")
func TestResume(t *testing.T){data:=[]byte("real download engine test");hash:=fmt.Sprintf("%x",md5.Sum(data));s:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.Header.Get("Range")!="bytes=4-"{t.Errorf("range: %s",r.Header.Get("Range"))};w.Header().Set("Content-Range",fmt.Sprintf("bytes 4-%d/%d",len(data)-1,len(data)));w.WriteHeader(206);w.Write(data[4:])}));defer s.Close();root:=t.TempDir();os.WriteFile(filepath.Join(root,"game.part"),data[:4],0644);var n atomic.Int64;e:=Entry{"game",s.URL,hash,int64(len(data))};if err:=download(context.Background(),root,e,nil,&n);err!=nil{t.Fatal(err)};if !verify(filepath.Join(root,"game"),e){t.Fatal("invalid output")}}
func TestIndex(t *testing.T){m,e:=parseIndex([]byte("0,3,900150983cd24fb0d6963f7d28e17f72,game.exe,/game.exe,\n"),false);if e!=nil||len(m.Files)!=1||m.Files[0].Size!=3{t.Fatal(m,e)};if _,e=safe(t.TempDir(),"../escape");e==nil{t.Fatal("traversal accepted")}}
func TestRangeIgnored(t *testing.T){data:=[]byte("abcdef");s:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Write(data)}));defer s.Close();root:=t.TempDir();os.WriteFile(filepath.Join(root,"x.part"),[]byte("bad"),0644);var n atomic.Int64;e:=Entry{"x",s.URL,fmt.Sprintf("%x",md5.Sum(data)),6};if err:=download(context.Background(),root,e,nil,&n);err!=nil{t.Fatal(err)};if !verify(filepath.Join(root,"x"),e){t.Fatal("invalid output")}}

func TestPerFileIndexCounts(t *testing.T) {
	const file = "0,3,900150983cd24fb0d6963f7d28e17f72,data/game.exe,/game.exe,\n"
	m, err := parseIndex([]byte("\n2,1,1,\n0,0,0,data,\n"+file), false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Archive != nil || len(m.Directories) != 1 || len(m.Files) != 1 ||
		m.Directories[0] != "data" || m.Files[0].Name != "data/game.exe" {
		t.Fatalf("unexpected per-file index: %+v", m)
	}
	for _, raw := range []string{"2,bad,1,\n" + file, file + "2,1,1,\n"} {
		if _, err := parseIndex([]byte(raw), false); err == nil {
			t.Fatalf("accepted unexpected header in %q", raw)
		}
	}
}
