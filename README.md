# API GO 

## ENABLE ROOT SSH
```bash
su
sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config
/etc/init.d/ssh restart
```

## DEPENDENCIES
```bash
su
apt update
apt -y install vim curl wget screen mc git unzip net-tools sox ufw
```

## FIREWALL
```bash
ufw  --force enable
ufw default deny incoming
ufw allow ssh
ufw allow 8080/tcp
ufw status
```

## INSTALL GO (GOLANG)
```bash
cd /usr/src/
wget https://go.dev/dl/go1.19.2.linux-amd64.tar.gz -O /usr/src/go.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go.linux-amd64.tar.gz
```

### CUSTOM GO
```bash
cat >> /root/.bashrc <<ENDLINE

#golang
export GOROOT=/usr/local/go
export PATH=$PATH:/usr/local/go/bin
export GOPATH=/root/go
export GOBIN=/root/go/bin

ENDLINE
```

```bash
source  /root/.bashrc
```

```bash
mkdir -p /root/go/{bin,pkg,src}
```

## PROJECT

```bash
mkdir -p /root/go/src/joinwavtostereo
mkdir /home/rec
```

```bash
vim /root/go/src/joinwavtostereo/main.go

```code
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
```

### STARTING PROJECT
```bash
cd /root/go/src/joinwavtostereo
go mod init joinwavtostereo
go mod tidy
```

### RUN DEVELOPER
```bash
cd /root/go/src/joinwavtostereo
go run main.go
```

### BUILD BINARY
```bash
cd /root/go/src/joinwavtostereo
go build -o joinwavtostereo main.go
```


### SYSTEMCTL - DEBIAN
```bash
cd /root/go/src/joinwavtostereo
rm -fr /usr/local/bin/joinwavtostereo
mv /root/go/src/joinwavtostereo/joinwavtostereo /usr/local/bin/joinwavtostereo

cat > /etc/rc.local <<ENDLINE
#!/bin/sh -e
#
# rc.local
#
# This script is executed at the end of each multiuser runlevel.
# Make sure that the script will "exit 0" on success or any other
# value on error.
#
# In order to enable or disable this script just change the execution
# bits.
#
# By default this script does nothing.
/usr/local/bin/joinwavtostereo &
exit 0
ENDLINE

chmod +x /etc/rc.local
systemctl daemon-reload
systemctl start rc-local
systemctl status rc-local

```

### Testing 
```bash
netstat -putan | grep 8080
```

```bash
curl --location 'http://192.168.1.73:8080/upload' -F 'audio1=@/Users/lordbasex/go/src/mono-to-stereo-32bit/audio1.wav' -F 'audio2=@/Users/lordbasex/go/src/mono-to-stereo-32bit/audio2.wav' -F "name=blabla-123421.1.wav"
```

```bash
ls -la /home/rec
total 196
drwxrwxrwx 2 root root   4096 May 30 10:11 .
drwxr-xr-x 4 root root   4096 May 30 08:24 ..
-rw-r--r-- 1 root root 192080 May 30 10:26 blabla-123421.1.wav
```


## ASTERISK MOD DIALPLAN
```bash
echo "

#include extensions_iperfex.comf
" >> /etc/asterisk/extensions_override_issabel.conf
```

## extensions_iperfex.comf
```bash
cat /etc/asterisk/extensions_iperfex.comf
[sub-record-check]
include => sub-record-check-custom
exten => s,1,NoOp(---- IPERFEX ----)
exten => s,n,Set(REC_POLICY_MODE_SAVE=${REC_POLICY_MODE})
exten => s,n,GotoIf($["${BLINDTRANSFER}" = ""]?check)
exten => s,n,ResetCDR()
exten => s,n,GotoIf($["${REC_STATUS}" != "RECORDING"]?check)
exten => s,n,Set(AUDIOHOOK_INHERIT(MixMonitor)=yes)
exten => s,n,MixMonitor(${MIXMON_DIR}${YEAR}/${MONTH}/${DAY}/${CALLFILENAME}.${MIXMON_FORMAT},a,${MIXMON_POST})
exten => s,n(check),Set(__MON_FMT=${IF($["${MIXMON_FORMAT}"="wav49"]?WAV:${MIXMON_FORMAT})})
exten => s,n,GotoIf($["${REC_STATUS}"!="RECORDING"]?next)
exten => s,n,Set(CDR(recordingfile)=${CALLFILENAME}.${MON_FMT})
exten => s,n,Return()
exten => s,n(next),ExecIf($[!${LEN(${ARG1})}]?Return())
exten => s,n,ExecIf($["${REC_POLICY_MODE}"="" & "${ARG3}"!=""]?Set(__REC_POLICY_MODE=${ARG3}))
exten => s,n,GotoIf($["${REC_STATUS}"!=""]?${ARG1},1)
exten => s,n,Set(__REC_STATUS=INITIALIZED)
exten => s,n,Set(NOW=${EPOCH})
exten => s,n,Set(__DAY=${STRFTIME(${NOW},,%d)})
exten => s,n,Set(__MONTH=${STRFTIME(${NOW},,%m)})
exten => s,n,Set(__YEAR=${STRFTIME(${NOW},,%Y)})
exten => s,n,Set(__TIMESTR=${YEAR}${MONTH}${DAY}-${STRFTIME(${NOW},,%H%M%S)})
exten => s,n,Set(__FROMEXTEN=${IF($[${LEN(${AMPUSER})}]?${AMPUSER}:${IF($[${LEN(${REALCALLERIDNUM})}]?${REALCALLERIDNUM}:unknown)})})
exten => s,n,Set(__CALLFILENAME=${ARG1}-${ARG2}-${FROMEXTEN}-${TIMESTR}-${UNIQUEID})
exten => s,n,Goto(${ARG1},1)

exten => rg,1,GosubIf($["${REC_POLICY_MODE}"="always"]?record,1(${EXTEN},${REC_POLICY_MODE},${FROMEXTEN}))
exten => rg,n,Return()

exten => force,1,GosubIf($["${REC_POLICY_MODE}"="always"]?record,1(${EXTEN},${REC_POLICY_MODE},${FROMEXTEN}))
exten => force,n,Return()

exten => q,1,GosubIf($["${REC_POLICY_MODE}"="always"]?recq,1(${EXTEN},${ARG2},${FROMEXTEN}))
exten => q,n,Return()

exten => out,1,ExecIf($["${REC_POLICY_MODE}"=""]?Set(__REC_POLICY_MODE=${DB(AMPUSER/${FROMEXTEN}/recording/out/external)}))
exten => out,n,GosubIf($["${REC_POLICY_MODE}"="always"]?record,1(exten,${ARG2},${FROMEXTEN}))
exten => out,n,Return()

exten => exten,1,GotoIf($["${REC_POLICY_MODE}"!=""]?callee)
exten => exten,n,Set(__REC_POLICY_MODE=${IF($[${LEN(${FROM_DID})}]?${DB(AMPUSER/${ARG2}/recording/in/external)}:${DB(AMPUSER/${ARG2}/recording/in/internal)})})
exten => exten,n,GotoIf($["${REC_POLICY_MODE}"="dontcare"]?caller)
exten => exten,n,GotoIf($["${DB(AMPUSER/${FROMEXTEN}/recording/out/internal)}"="dontcare" | "${FROM_DID}"!=""]?callee)
exten => exten,n,ExecIf($[${LEN(${DB(AMPUSER/${FROMEXTEN}/recording/priority)})}]?Set(CALLER_PRI=${DB(AMPUSER/${FROMEXTEN}/recording/priority)}):Set(CALLER_PRI=0))
exten => exten,n,ExecIf($[${LEN(${DB(AMPUSER/${ARG2}/recording/priority)})}]?Set(CALLEE_PRI=${DB(AMPUSER/${ARG2}/recording/priority)}):Set(CALLEE_PRI=0))
exten => exten,n,GotoIf($["${CALLER_PRI}"="${CALLEE_PRI}"]?${REC_POLICY}:${IF($[${CALLER_PRI}>${CALLEE_PRI}]?caller:callee)})
exten => exten,n(callee),GosubIf($["${REC_POLICY_MODE}"="always"]?record,1(${EXTEN},${ARG2},${FROMEXTEN}))
exten => exten,n,Return()
exten => exten,n(caller),Set(__REC_POLICY_MODE=${DB(AMPUSER/${FROMEXTEN}/recording/out/internal)})
exten => exten,n,GosubIf($["${REC_POLICY_MODE}"="always"]?record,1(${EXTEN},${ARG2},${FROMEXTEN}))
exten => exten,n,Return()

exten => conf,1,Gosub(recconf,1(${EXTEN},${ARG2},${ARG2}))
exten => conf,n,Return()

exten => page,1,GosubIf($["${REC_POLICY_MODE}"="always"]?recconf,1(${EXTEN},${ARG2},${FROMEXTEN}))
exten => page,n,Return()

exten => record,1,Set(AUDIOHOOK_INHERIT(MixMonitor)=yes)
exten => record,n,MixMonitor(${MIXMON_DIR}${YEAR}/${MONTH}/${DAY}/${CALLFILENAME}.${MIXMON_FORMAT},b,${MIXMON_POST})

exten => record,n,Monitor(wav,${MIXMON_DIR}${YEAR}/${MONTH}/${DAY}/${CALLFILENAME}-A,b)
exten => record,n,Set(__CHANNEL_IN=${MIXMON_DIR}${YEAR}/${MONTH}/${DAY}/${CALLFILENAME}-A-in.wav)
exten => record,n,Set(__CHANNEL_OUT=${MIXMON_DIR}${YEAR}/${MONTH}/${DAY}/${CALLFILENAME}-A-out.wav)
exten => record,n,Set(__CHANNEL_NAME_FINAL=${CALLFILENAME}-A-STEREO.wav)
exten => record,n,Set(CHANNEL(hangup_handler_push)=hangup-iperfex,h,1)

exten => record,n,Set(__REC_STATUS=RECORDING)
exten => record,n,Set(CDR(recordingfile)=${CALLFILENAME}.${MON_FMT})
exten => record,n,Return()

exten => recq,1,Set(AUDIOHOOK_INHERIT(MixMonitor)=yes)
exten => recq,n,Set(MONITOR_FILENAME=${MIXMON_DIR}${YEAR}/${MONTH}/${DAY}/${CALLFILENAME})
exten => recq,n,MixMonitor(${MONITOR_FILENAME}.${MIXMON_FORMAT},${MONITOR_OPTIONS},${MIXMON_POST})
exten => recq,n,Set(__REC_STATUS=RECORDING)
exten => recq,n,Set(CDR(recordingfile)=${CALLFILENAME}.${MON_FMT})
exten => recq,n,Return()

exten => recconf,1,Set(__CALLFILENAME=${IF($[${MEETME_INFO(parties,${ARG2})}]?${DB(RECCONF/${ARG2})}:${ARG1}-${ARG2}-${ARG3}-${TIMESTR}-${UNIQUEID})})
exten => recconf,n,ExecIf($[!${MEETME_INFO(parties,${ARG2})}]?Set(DB(RECCONF/${ARG2})=${CALLFILENAME}))
exten => recconf,n,Set(MEETME_RECORDINGFILE=${IF($[${LEN(${MIXMON_DIR})}]?${MIXMON_DIR}:${ASTSPOOLDIR}/monitor/)}${YEAR}/${MONTH}/${DAY}/${CALLFILENAME})
exten => recconf,n,Set(MEETME_RECORDINGFORMAT=${MIXMON_FORMAT})
exten => recconf,n,ExecIf($["${REC_POLICY_MODE}"!="always"]?Return())
exten => recconf,n,Set(__REC_STATUS=RECORDING)
exten => recconf,n,Set(CDR(recordingfile)=${CALLFILENAME}.${MON_FMT})
exten => recconf,n,Return()

;--== end of [sub-record-check] ==--;


[hangup-iperfex]
exten => h,1,NoOp(---- IPERFEX -----)
 same => n,StopMonitor()
 same => n,NoOP(UNIQUEID: startCALL: ${CDR(start)} endCALL: ${CDR(end)} durationCALL: ${CDR(duration)})
 same => n,NoOp(CHANNEL_OUT CLIENTE: /var/spool/asterisk/monitor/${CHANNEL_OUT})
 same => n,NoOp(CHANNEL_INT AGENTE: /var/spool/asterisk/monitor/${CHANNEL_IN})
 same => n,AGI(iperfex2.agi,/var/spool/asterisk/monitor/${CHANNEL_OUT},/var/spool/asterisk/monitor/${CHANNEL_IN},/var/spool/asterisk/monitor/${CHANNEL_NAME_FINAL})
 same => n,Return()
```
