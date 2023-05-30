package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
)

var DebugNow bool = true
var PathAudioStereo string = "/home/rec"

type Msg struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

// goDotEnvVariable funcion que trae variables de entorno del sistema operativo.
func goDotEnvVariable(key string) string {
	return os.Getenv(key)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// apt install sox
	// curl --location 'http://172.16.0.70:8080/upload' -F 'audio1=@/Users/lordbasex/go/src/mono-to-stereo-32bit/audio1.wav' -F 'audio2=@/Users/lordbasex/go/src/mono-to-stereo-32bit/audio2.wav' -F "name=blabla-123421.1.wav"

	DebugNow, _ = strconv.ParseBool(goDotEnvVariable("DEBUG"))
	PathAudioStereo = goDotEnvVariable("PATHAUDIO")
	if PathAudioStereo == "" {
		PathAudioStereo = "/home/rec"
	}

	router := mux.NewRouter()
	router.HandleFunc("/upload", handleUpload).Methods("POST")

	//Start Service
	log.Println("API: Start - http://0.0.0.0:8080")
	log.Printf("DebugNow: %v", DebugNow)
	log.Printf("PathAudioStereo: %s", PathAudioStereo)
	log.Fatal(http.ListenAndServe(":8080", router))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(50 << 20) // Tamaño máximo de 50MB para los archivos

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Msg{
			Status: http.StatusInternalServerError,
			Msg:    "Maximum size of 10MB for files",
		})
		return
	}

	// Obtener el nombre del archivo del parámetro "name" en el cuerpo de la solicitud
	name := r.FormValue("name")
	if name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Msg{
			Status: http.StatusInternalServerError,
			Msg:    "Parameter 'name' not provided",
		})
		return
	}

	file1, handler1, err := r.FormFile("audio1")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Msg{
			Status: http.StatusInternalServerError,
			Msg:    "Could not read file 'audio1'",
		})
		return
	}
	defer file1.Close()

	file2, handler2, err := r.FormFile("audio2")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Msg{
			Status: http.StatusInternalServerError,
			Msg:    "Could not read file 'audio2'",
		})
		return
	}
	defer file2.Close()

	// Guardar archivo 1
	filename1 := handler1.Filename
	err = saveFile(file1, filename1)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Msg{
			Status: http.StatusInternalServerError,
			Msg:    "Error saving file 'audio1'",
		})
		return
	}

	// Guardar archivo 2
	filename2 := handler2.Filename
	err = saveFile(file2, filename2)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Msg{
			Status: http.StatusInternalServerError,
			Msg:    "Error saving file 'audio2'",
		})
		return
	}

	//crea nuevo archivo.
	err = monoToStereo(name, filename1, filename2)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Msg{
			Status: http.StatusInternalServerError,
			Msg:    "error could not join the files.",
		})
		os.Remove("/tmp/" + filename1)
		os.Remove("/tmp/" + filename2)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Msg{
		Status: http.StatusOK,
		Msg:    "processed audio.",
	})
	return

}

func saveFile(file multipart.File, filename string) error {
	// Obtener la ruta absoluta para guardar los archivos en el directorio actual
	absPath, err := filepath.Abs("/tmp/")
	if err != nil {
		return err
	}

	if DebugNow {
		log.Printf("PATH SAVEFILE: %s/%s", absPath, filename)
	}

	// Crear el archivo en disco
	out, err := os.Create(filepath.Join(absPath, filename))
	if err != nil {
		return err
	}
	defer out.Close()

	// Copiar el contenido del archivo subido al archivo en disco
	_, err = io.Copy(out, file)
	if err != nil {
		return err
	}

	return nil
}

func monoToStereo(name, filename1, filename2 string) error {

	if DebugNow {
		log.Printf("Join WAV To Stereo filename: %s/%s", PathAudioStereo, name)
	}

	err := os.MkdirAll(PathAudioStereo, 0755)
	if err != nil {
		return err
	}

	// Verificar la existencia de los archivos filename1 y filename2
	file1Exists, err := fileExists("/tmp/" + filename1)
	if err != nil {
		return err
	}
	if !file1Exists {
		return fmt.Errorf("el archivo %s no existe", filename1)
	}

	file2Exists, err := fileExists("/tmp/" + filename2)
	if err != nil {
		return err
	}
	if !file2Exists {
		return fmt.Errorf("el archivo %s no existe", filename2)
	}

	// sox -M -c 1 audio1.wav -c 1 audio2.wav -b 32 output1.wav
	cmd := exec.Command("sox", "-M", "-c", "1", "/tmp/"+filename1, "-c", "1", "/tmp/"+filename2, "-b", "32", PathAudioStereo+"/"+name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	os.Remove("/tmp/" + filename1)
	os.Remove("/tmp/" + filename2)
	return nil
}

func fileExists(filename string) (bool, error) {
	_, err := os.Stat(filename)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
