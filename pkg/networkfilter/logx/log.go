package logx

import (
	nested "github.com/antonfisher/nested-logrus-formatter"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/pkg/errors"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

// 全局变量
var (
	Logger       *logrus.Logger
	instanceName string
)

const (
	defaultPod          = "localhost"
	defaultLogName      = "ncs.log"
	defaultRotationSize = 50 * 1000 * 1000
	defaultLogPath      = "/var/log/networkfilter"
)

func GetMyPodName() string {
	instanceName = defaultPod
	return instanceName
}

func NewLogger() *logrus.Logger {
	if Logger != nil {
		return Logger
	}
	Logger = logrus.New()

	Logger.SetFormatter(&nested.Formatter{
		TimestampFormat: time.RFC3339,
		CallerFirst:     true,
	})
	Logger.SetLevel(logrus.DebugLevel)
	Logger.SetOutput(os.Stdout)

	//日志切割
	//path := "./logs/idmsagent/error.log" // 动态获取
	var path = defaultLogPath
	var name = defaultLogName
	var rotateSize = defaultRotationSize

	writerErr, err := rotatelogs.New(
		path+"/"+"/"+name+".%Y%m%d",
		rotatelogs.WithLinkName(path),              // 生成软链，指向最新日志文件
		rotatelogs.WithRotationTime(5*time.Second), // 日志切割时间间隔
		rotatelogs.WithMaxAge(1*time.Minute),       //clear 清理的时间
		rotatelogs.ForceNewFile(),
		rotatelogs.WithRotationSize(int64(rotateSize)*1000), //设置文件大小切分单位byte,这里用200Ml
	)
	if err != nil {
		Logger.Errorf("config local file system logger error. %+v", errors.WithStack(err))
	}

	//pathInfo := "./logs/idmsagent/info.log" // 动态获取,各个级别，合并成一个日志文件，对于Error或者打调用栈的，可分开
	writerInfo, _ := rotatelogs.New(
		path+"/"+"/"+name+".%Y%m%d",
		rotatelogs.WithLinkName(path),              // 生成软链，指向最新日志文件
		rotatelogs.WithRotationTime(5*time.Second), // 日志切割时间间隔
		rotatelogs.WithMaxAge(1*time.Minute),       //clear 清理的时间
		rotatelogs.ForceNewFile(),
		rotatelogs.WithRotationSize(int64(rotateSize)*1000),
	)

	//debug := "./logs/idmsagent/debug.log" // 动态获取
	_, _ = rotatelogs.New(
		path+"/"+"/"+name+".%Y%m%d",
		rotatelogs.WithLinkName(path),              // 生成软链，指向最新日志文件
		rotatelogs.WithRotationTime(5*time.Second), // 日志切割时间间隔
		rotatelogs.WithMaxAge(1*time.Minute),       //clear 清理的时间
		rotatelogs.ForceNewFile(),
		rotatelogs.WithRotationSize(int64(rotateSize)*1000),
	)

	lfHook := lfshook.NewHook(lfshook.WriterMap{
		logrus.DebugLevel: writerInfo, // 为不同级别设置不同的输出目的,写到一个各个级别写入到一个log文件中，因此复用RotateLogs对象writerInfo
		logrus.InfoLevel:  writerInfo, // 为不同级别设置不同的输出目的,写到一个各个级别写入到一个log文件中，因此复用RotateLogs对象writerInfo
		logrus.WarnLevel:  writerInfo, // 为不同级别设置不同的输出目的,写到一个各个级别写入到一个log文件中，因此复用RotateLogs对象writerInfo
		logrus.ErrorLevel: writerInfo, // 为不同级别设置不同的输出目的,写到一个各个级别写入到一个log文件中，因此复用RotateLogs对象writerInfo
		logrus.FatalLevel: writerErr,
		logrus.PanicLevel: writerInfo, // 为不同级别设置不同的输出目的,写到一个各个级别写入到一个log文件中，因此复用RotateLogs对象writerInfo
	}, nil)
	Logger.AddHook(lfHook)

	return Logger
}

func Debug(args ...interface{}) {
	Logger.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

func Info(args ...interface{}) {
	Logger.Info(args...)
}

func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

func Warn(args ...interface{}) {
	Logger.Warn(args...)
}

func Warningf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
}

func Error(args ...interface{}) {
	Logger.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
}
