package gocbcore

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	defaultServerVersion = "5.1.0"
)

type testLogger struct {
	Parent   Logger
	LogCount []uint64
}

func (logger *testLogger) Log(level LogLevel, offset int, format string, v ...interface{}) error {
	if level >= 0 && level < LogMaxVerbosity {
		atomic.AddUint64(&logger.LogCount[level], 1)
	}

	return logger.Parent.Log(level, offset+1, fmt.Sprintf("[%s] ", logLevelToString(level))+format, v...)
}

func createTestLogger() *testLogger {
	return &testLogger{
		Parent:   VerboseStdioLogger(),
		LogCount: make([]uint64, LogMaxVerbosity),
	}
}

func envFlagString(envName, name, value, usage string) *string {
	envValue := os.Getenv(envName)
	if envValue != "" {
		value = envValue
	}
	return flag.String(name, value, usage)
}

func envFlagBool(envName, name string, value bool, usage string) *bool {
	envValue := os.Getenv(envName)
	if envValue != "" {
		if envValue == "0" {
			value = false
		} else if strings.ToLower(envValue) == "false" {
			value = false
		} else {
			value = true
		}
	}
	return flag.Bool(name, value, usage)
}

func TestMain(m *testing.M) {
	initialGoroutineCount := runtime.NumGoroutine()

	connStr := envFlagString("GCBCONNSTR", "connstr", "",
		"Connection string to run tests with")
	bucketName := envFlagString("GCBBUCKET", "bucket", "default",
		"The bucket to use to test against")
	memdBucketName := envFlagString("GCBMEMDBUCKET", "memd-bucket", "memd",
		"The memd bucket to use to test against")
	username := envFlagString("GCBUSER", "user", "",
		"The username to use to authenticate when using a real server")
	password := envFlagString("GCBPASS", "pass", "",
		"The password to use to authenticate when using a real server")
	clusterVersionStr := envFlagString("GCBCVER", "cluster-version", defaultServerVersion,
		"The server version being tested against (major.minor.patch.build_edition)")
	scopeName := envFlagString("GCBSCOPE", "scope-name", "",
		"The scope name to use to test with collections")
	collectionName := envFlagString("GCBCOLL", "collection-name", "",
		"The collection name to use to test with collections")
	featuresToTest := envFlagString("GCBFEAT", "features", "",
		"The features that should be tested")
	disableLogger := envFlagBool("GCBNOLOG", "disable-logger", false,
		"Whether or not to disable the logger")
	flag.Parse()

	clusterVersion, err := nodeVersionFromString(*clusterVersionStr)
	if err != nil {
		panic("failed to parse specified cluster version")
	}

	var featureFlags []TestFeatureFlag
	featureFlagStrs := strings.Split(*featuresToTest, ",")
	for _, featureFlagStr := range featureFlagStrs {
		if len(featureFlagStr) == 0 {
			continue
		}

		if featureFlagStr[0] == '+' {
			featureFlags = append(featureFlags, TestFeatureFlag{
				Enabled: true,
				Feature: TestFeatureCode(featureFlagStr[1:]),
			})
			continue
		} else if featureFlagStr[0] == '-' {
			featureFlags = append(featureFlags, TestFeatureFlag{
				Enabled: false,
				Feature: TestFeatureCode(featureFlagStr[1:]),
			})
			continue
		}

		panic("failed to parse specified feature codes")
	}

	var logger *testLogger
	if !*disableLogger {
		// Set up our special logger which logs the log level count
		logger = createTestLogger()
		SetLogger(logger)
	}

	globalTestConfig = &TestConfig{
		ConnStr:        *connStr,
		BucketName:     *bucketName,
		MemdBucketName: *memdBucketName,
		ScopeName:      *scopeName,
		CollectionName: *collectionName,
		Username:       *username,
		Password:       *password,
		ClusterVersion: clusterVersion,
		FeatureFlags:   featureFlags,
	}

	result := m.Run()

	if logger != nil {
		log.Printf("Log Messages Emitted:")
		var preLogTotal uint64
		for i := 0; i < int(LogMaxVerbosity); i++ {
			count := atomic.LoadUint64(&logger.LogCount[i])
			preLogTotal += count
			log.Printf("  (%s): %d", logLevelToString(LogLevel(i)), count)
		}

		abnormalLogCount := atomic.LoadUint64(&logger.LogCount[LogError]) + atomic.LoadUint64(&logger.LogCount[LogWarn])
		if abnormalLogCount > 0 {
			log.Printf("Detected unexpected logging, failing")
			result = 1
		}

		time.Sleep(1 * time.Second)

		log.Printf("Post sleep log Messages Emitted:")
		var postLogTotal uint64
		for i := 0; i < int(LogMaxVerbosity); i++ {
			count := atomic.LoadUint64(&logger.LogCount[i])
			postLogTotal += count
			log.Printf("  (%s): %d", logLevelToString(LogLevel(i)), count)
		}

		if preLogTotal != postLogTotal {
			log.Printf("Detected unexpected logging after agent closed, failing")
			result = 1
		}
	}

	// Loop for at most a second checking for goroutines leaks, this gives any HTTP goroutines time to shutdown
	start := time.Now()
	var finalGoroutineCount int
	for time.Now().Sub(start) <= 1*time.Second {
		runtime.Gosched()
		finalGoroutineCount = runtime.NumGoroutine()
		if finalGoroutineCount == initialGoroutineCount {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if finalGoroutineCount != initialGoroutineCount {
		log.Printf("Detected a goroutine leak (%d before != %d after), failing", initialGoroutineCount, finalGoroutineCount)
		pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		result = 1
	} else {
		log.Printf("No goroutines appear to have leaked (%d before == %d after)", initialGoroutineCount, finalGoroutineCount)
	}

	os.Exit(result)
}
