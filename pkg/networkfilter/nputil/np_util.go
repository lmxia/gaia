package nputil

import (
	"bytes"
	"errors"
	"github.com/lmxia/gaia/pkg/networkfilter/logx"
	"hash/fnv"
	"math/rand"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"
	"unsafe"
)

type LocalStatus uint32

const (
	LocalStatus_Invalid LocalStatus = 0
	LocalStatus_Running LocalStatus = 1
	LogSwitch                       = "Off"
)

func TraceInfo(inputString string) {
	if LogSwitch == "Off" {
		return
	}
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Info("routineId:" + routineId + "," + "func:" + funcName + "," + "input:" + inputString)
}

func TraceInfoAlwaysPrint(inputString string) {
	LogEnable := os.Getenv("NetFilterLogEnable")
	if len(LogEnable) == 0 || LogEnable != "ON" {
		return
	}
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Info("routineId:" + routineId + "," + "func:" + funcName + "," + "input:" + inputString)
}

func TraceInfoBegin(inputString string) {
	if LogSwitch == "Off" {
		return
	}
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Info("routineId:" + routineId + "," + "func:" + funcName + "," + " Begin. input:" + inputString)
}

func TraceInfoEnd(inputString string) {
	if LogSwitch == "Off" {
		return
	}
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Info("routineId:" + routineId + "," + "func:" + funcName + "," + " End input:" + inputString)
	return
}

func TraceError(err error) {
	if LogSwitch == "Off" {
		return
	}
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Error("routineId:"+routineId+","+"func:"+funcName+","+" err:", err)
	return
}

func TraceErrorString(errString string) {
	if LogSwitch == "Off" {
		return
	}
	err := errors.New(errString)
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Error("routineId:"+routineId+","+"func:"+funcName+","+" err:", err)
	return
}

func TraceErrorWithStack(err error) {
	if LogSwitch == "Off" {
		return
	}
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	routineId := GetGIDString()
	logx.Error("routineId:"+routineId+","+"func:"+funcName+","+" err:", err)
	logx.Error(string(debug.Stack()))
	return
}

func TraceErrorStringWithStack(errString string) {
	if LogSwitch == "Off" {
		return
	}
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

//生成count个[start,end)结束的不重复的随机数
func GenerateRandomIndex(start int, end int, count int) []int {
	//范围检查
	if end < start || (end-start) < count {
		return nil
	}
	//存放结果的slice
	nums := make([]int, 0)
	//随机数生成器，加入时间戳保证每次生成的随机数不一样
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(nums) < count {
		//生成随机数
		num := r.Intn((end - start)) + start
		//查重
		exist := false
		for _, v := range nums {
			if v == num {
				exist = true
				break
			}
		}
		if !exist {
			nums = append(nums, num)
		}
	}
	return nums
}
