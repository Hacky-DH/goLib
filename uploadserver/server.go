package main

import (
	"io"
	"log"
	"net/http"
	"os"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		//parse the multipart form in the request
		err := r.ParseMultipartForm(100000)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// use hostname as the prefix of file name
		host := r.FormValue("host")
		if len(host) > 0 {
			host = host + "_"
		}
		files := r.MultipartForm.File["uploadfile"]
		for i, _ := range files {
			file, err := files[i].Open()
			defer file.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			dst, err := os.Create("./upload/" + host + files[i].Filename)
			defer dst.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if _, err := io.Copy(dst, file); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			log.Println("uploaded " + files[i].Filename)
		}
		if len(files) == 0 {
			http.Error(w, "no file upload", http.StatusInternalServerError)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func main() {
	os.Mkdir("upload", 0777)
	os.Mkdir("staticfile", 0777)
	http.HandleFunc("/upload", uploadHandler)
	http.Handle("/staticfile/", http.StripPrefix("/staticfile/",
		http.FileServer(http.Dir("./staticfile"))))

	log.Fatal(http.ListenAndServe(":7778", nil))
}
