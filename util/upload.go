package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

const (
	WEBSITE       = "https://blueimp.github.io/jQuery-File-Upload/"
	MIN_FILE_SIZE = 1 // bytes
	// Max file size is memcache limit (1MB) minus key size minus overhead:
	MAX_FILE_SIZE     = 999000 // bytes
	IMAGE_TYPES       = "image/(gif|p?jpeg|(x-)?png)"
	ACCEPT_FILE_TYPES = IMAGE_TYPES
	THUMB_MAX_WIDTH   = 80
	THUMB_MAX_HEIGHT  = 80
	EXPIRATION_TIME   = 300 // seconds
	// If empty, only allow redirects to the referer protocol+host.
	// Set to a regexp string for custom pattern matching:
	REDIRECT_ALLOW_TARGET = ""
	HOST_NAME             = "iwshoes.cn"
	IMAGE_DIRECTORY       = "/var/www/image/"
)

var (
	imageTypes      = regexp.MustCompile(IMAGE_TYPES)
	acceptFileTypes = regexp.MustCompile(ACCEPT_FILE_TYPES)
	thumbSuffix     = "." + fmt.Sprint(THUMB_MAX_WIDTH) + "x" +
		fmt.Sprint(THUMB_MAX_HEIGHT)
)

func escape(s string) string {
	return strings.Replace(url.QueryEscape(s), "+", "%20", -1)
}

func extractKey(r *http.Request) string {
	r.ParseForm()
	return r.FormValue("id")
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type FileInfo struct {
	Key        string `json:"-"`
	Url        string `json:"url,omitempty"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Error      string `json:"error,omitempty"`
	DeleteUrl  string `json:"deleteUrl,omitempty"`
	DeleteType string `json:"deleteType,omitempty"`
}

func (fi *FileInfo) ValidateType() (valid bool) {
	if acceptFileTypes.MatchString(fi.Type) {
		return true
	}
	fi.Error = "Filetype not allowed"
	return false
}

func (fi *FileInfo) ValidateSize() (valid bool) {
	if fi.Size < MIN_FILE_SIZE {
		fi.Error = "File is too small"
	} else if fi.Size > MAX_FILE_SIZE {
		fi.Error = "File is too big"
	} else {
		return true
	}
	return false
}

func (fi *FileInfo) CreateUrls(r *http.Request) {
	u := &url.URL{
		Scheme: r.URL.Scheme,
		Host:   HOST_NAME,
		Path:   "/image/" + fi.Key,
	}
	uString := u.String()
	fi.Url = uString + escape(string(fi.Name))
	u.Path = "/upload?id=" + fi.Key[:len(fi.Key)-1]
	fi.DeleteUrl = u.String()
	fi.DeleteType = "DELETE"
}

func (fi *FileInfo) SetKey(checksum uint32) {
	fi.Key = escape(fmt.Sprint(checksum)) + "/"
}

func handleUpload(r *http.Request, p *multipart.Part) (fi *FileInfo) {
	fi = &FileInfo{
		Name: p.FileName(),
		Type: p.Header.Get("Content-Type"),
	}
	if !fi.ValidateType() {
		return
	}
	defer func() {
		if rec := recover(); rec != nil {
			log.Println(rec)
			fi.Error = rec.(error).Error()
		}
	}()

	var buffer bytes.Buffer
	hash := crc32.NewIEEE()
	mw := io.MultiWriter(&buffer, hash)
	lr := &io.LimitedReader{R: p, N: MAX_FILE_SIZE + 1}
	_, err := io.Copy(mw, lr)
	check(err)
	fi.Size = MAX_FILE_SIZE + 1 - lr.N
	if !fi.ValidateSize() {
		return
	}
	fi.SetKey(hash.Sum32())

	filePath := IMAGE_DIRECTORY + fi.Key
	err = os.MkdirAll(filePath, 0777)
	check(err)
	f, err := os.OpenFile(filePath+fi.Name, os.O_WRONLY|os.O_CREATE, 0666)
	check(err)
	defer f.Close()
	io.Copy(f, &buffer)

	fi.CreateUrls(r)
	return
}

func getFormValue(p *multipart.Part) string {
	var b bytes.Buffer
	io.CopyN(&b, p, int64(1<<20)) // Copy max: 1 MiB
	return b.String()
}

func handleUploads(r *http.Request) (fileInfos []*FileInfo) {
	fileInfos = make([]*FileInfo, 0)
	mr, err := r.MultipartReader()
	check(err)
	r.Form, err = url.ParseQuery(r.URL.RawQuery)
	check(err)
	part, err := mr.NextPart()
	for err == nil {
		if name := part.FormName(); name != "" {
			if part.FileName() != "" {
				fileInfos = append(fileInfos, handleUpload(r, part))
			} else {
				r.Form[name] = append(r.Form[name], getFormValue(part))
			}
		}
		part, err = mr.NextPart()
	}
	return
}

func validateRedirect(r *http.Request, redirect string) bool {
	if redirect != "" {
		var redirectAllowTarget *regexp.Regexp
		if REDIRECT_ALLOW_TARGET != "" {
			redirectAllowTarget = regexp.MustCompile(REDIRECT_ALLOW_TARGET)
		} else {
			referer := r.Referer()
			if referer == "" {
				return false
			}
			refererUrl, err := url.Parse(referer)
			if err != nil {
				return false
			}
			redirectAllowTarget = regexp.MustCompile("^" + regexp.QuoteMeta(
				refererUrl.Scheme+"://"+refererUrl.Host+"/",
			))
		}
		return redirectAllowTarget.MatchString(redirect)
	}
	return false
}

func post(w http.ResponseWriter, r *http.Request) {
	result := make(map[string][]*FileInfo, 1)
	result["files"] = handleUploads(r)
	b, err := json.Marshal(result)
	check(err)
	if redirect := r.FormValue("redirect"); validateRedirect(r, redirect) {
		if strings.Contains(redirect, "%s") {
			redirect = fmt.Sprintf(
				redirect,
				escape(string(b)),
			)
		}
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	jsonType := "application/json"
	if strings.Index(r.Header.Get("Accept"), jsonType) != -1 {
		w.Header().Set("Content-Type", jsonType)
	}
	fmt.Fprintln(w, string(b))
}

func delete(w http.ResponseWriter, r *http.Request) {
	params, err := url.ParseQuery(r.URL.RawQuery)
	log.Println(params)
	key := params["id"][0]
	if key == "" {
		http.Error(w, "405 Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err = os.RemoveAll(IMAGE_DIRECTORY + key)
	result := make(map[string]bool, 1)
	result[key] = true
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(result)
	check(err)
	fmt.Fprintln(w, string(b))
}

func UploadHandle(w http.ResponseWriter, r *http.Request) {
	params, err := url.ParseQuery(r.URL.RawQuery)
	url.Parse("x")
	check(err)
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add(
		"Access-Control-Allow-Methods",
		"OPTIONS, HEAD, POST, DELETE",
	)
	w.Header().Add(
		"Access-Control-Allow-Headers",
		"Content-Type, Content-Range, Content-Disposition",
	)
	switch r.Method {
	case "OPTIONS", "HEAD":
		return
	case "POST":
		if len(params["_method"]) > 0 && params["_method"][0] == "DELETE" {
			delete(w, r)
		} else {
			post(w, r)
		}
	case "DELETE":
		delete(w, r)
	default:
		http.Error(w, "501 Not Implemented", http.StatusNotImplemented)
	}
}
