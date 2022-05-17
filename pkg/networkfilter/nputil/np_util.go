package nputil

import (
	"bytes"
	"errors"
	"github.com/lmxia/gaia/pkg/networkfilter/logx"
	"hash/fnv"
	"runtime"
	"runtime/debug"
	"strconv"
	"unsafe"
)

type LocalStatus uint32

const (
	LocalStatus_Invalid LocalStatus = 0
	LocalStatus_Running LocalStatus = 1
)

func TraceInfo(inputString string) {
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Info("routineId:" + routineId + "," + "func:" + funcName + "," + "input:" + inputString)
}

func TraceInfoBegin(inputString string) {
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Info("routineId:" + routineId + "," + "func:" + funcName + "," + " Begin. input:" + inputString)
}

func TraceInfoEnd(inputString string) {
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Info("routineId:" + routineId + "," + "func:" + funcName + "," + " End input:" + inputString)
	return
}

func TraceError(err error) {
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Error("routineId:"+routineId+","+"func:"+funcName+","+" err:", err)
	return
}

func TraceErrorString(errString string) {
	err := errors.New(errString)
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Error("routineId:"+routineId+","+"func:"+funcName+","+" err:", err)
	return
}

func TraceErrorWithStack(err error) {
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Error("routineId:"+routineId+","+"func:"+funcName+","+" err:", err)
	logx.Error(string(debug.Stack()))
	return
}

func TraceErrorStringWithStack(errString string) {
	err := errors.New(errString)
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Error("routineId:"+routineId+","+"func:"+funcName+","+" err:", err)
	logx.Error(string(debug.Stack()))
	return
}

func StringHash(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

func GetGIDString() string {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)

	gidString := strconv.FormatInt(int64(n), 10)
	return gidString
}

//Str2bytes  string to []byte
func Str2bytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

// bytes to str
func Bytes2str(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
