package main

import (
  "bytes"
  "crypto/tls"
  "encoding/base64"
  "encoding/json"
  "errors"
  "flag"
  "fmt"
  "io/ioutil"
  "net/http"
  "net/url"
  "os"
  "path/filepath"
  "regexp"
  "strconv"
  "strings"
  "sync"
  "unicode/utf8"

  "github.com/go-redis/redis"

  "golang.org/x/text/encoding/simplifiedchinese"
  "golang.org/x/text/transform"
)

type Config struct {
  AuthEnabled  bool
  DynamicToken bool
  LyricsDir    string
  Port         int
  RedisAddress string
  Token        string
  Version      string
  VersionFlag  bool
}

var config Config
var redisClient *redis.Client

func main() {
  config = initConfig()

  http.HandleFunc("/", rootHandler)

  http.HandleFunc("/lyrics", lyricsHandler)

  port := strconv.Itoa(config.Port)
  fmt.Printf("Server started on port %s\n", port)
  err := http.ListenAndServe(":"+port, nil)
  if err != nil {
    fmt.Printf("Server failed to start: %v\n", err)
  }
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
  w.WriteHeader(http.StatusOK)
}

func lyricsHandler(w http.ResponseWriter, r *http.Request) {
  if config.AuthEnabled {
    err := authenticateRequest(r)
    if err != nil {
      http.Error(w, err.Error(), http.StatusUnauthorized)
      return
    }
  }

  artist, title, err := extractParameters(r)
  if err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
  }

  lyrics, err := findLyrics(artist, title)
  if err != nil {
    http.Error(w, err.Error(), http.StatusNotFound)
    return
  }

  w.WriteHeader(http.StatusOK)
  _, _ = w.Write([]byte(lyrics))
}

func initRedisClient() *redis.Client {
  if redisClient == nil {
    redisClient = redis.NewClient(&redis.Options{
      Addr: config.RedisAddress,
    })
  }
  return redisClient
}

func authenticateRequest(r *http.Request) error {
  token := r.Header.Get("Authorization")
  token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))

  if token != "" && isTokenValid(token) {
    if config.DynamicToken {
      client := initRedisClient()

      result, err := client.Exists(token).Result()
      if err != nil {
        // 将 Redis 错误记录到日志
        fmt.Printf("Redis error: %v\n", err)
        return err
      }
      if result == 1 {
        return nil
      }
    } else {
      if token == config.Token {
        return nil
      }
    }
  }

  return errors.New("Unauthorized")
}

func isTokenValid(token string) bool {
  // 匹配正常字符的正则表达式
  regex := `^[A-Za-z0-9]{8,16}$`

  match, err := regexp.MatchString(regex, token)
  if err != nil {
    // 处理正则匹配错误
    return false
  }

  return match
}

func extractParameters(r *http.Request) (string, string, error) {
  vars := r.URL.Query()

  artist := vars.Get("artist")
  title := vars.Get("title")

  if artist == "" || title == "" {
    return "", "", errors.New("Missing required parameters \"artist\" or \"title\".")
  }

  return artist, title, nil
}

func findLyrics(artist, title string) (string, error) {
  var wg sync.WaitGroup
  var result string
  var err error

  // 从本地读取 lrc 文件
  wg.Add(1)
  go func(artist, title string) {
    defer wg.Done()

    if artist != "" && title != "" {
      filePath := filepath.Join(config.LyricsDir, artist+"-"+title+".lrc")
      fileContent, err := getFileContent(filePath)
      if err == nil {
        result = fileContent
      }
    }
  }(artist, title)

  wg.Add(1)
  go func(title string) {
    defer wg.Done()

    if title != "" {
      filePath := filepath.Join(config.LyricsDir, title+".lrc")
      fileContent, err := getFileContent(filePath)
      if err == nil {
        result = fileContent
      }
    }
  }(title)

  wg.Wait()

  if result != "" {
    return result, nil
  }

  // 从网络获取
  fileContent, err := getLyricsFromNet(title, artist)
  if err != nil {
    return "", errors.New("Lyrics not found.")
  }

  go saveLyricsToLrc(artist, title, fileContent)

  return fileContent, nil
}

func getFileContent(filePath string) (string, error) {
  file, err := os.Open(filePath)
  if err != nil {
    if os.IsNotExist(err) {
      return "", fmt.Errorf("file not found")
    }
    return "", err
  }
  defer file.Close()

  fileContent, err := ioutil.ReadAll(file)
  if err != nil {
    return "", err
  }

  return convertEncoding(string(fileContent))
}

func getLyricsFromNet(title, artist string) (string, error) {
  if title == "" {
    return "", nil
  }

  searcher := url.QueryEscape(title + artist)

  client := createHTTPClient()
  resp, err := client.Get("http://mobilecdn.kugou.com/api/v3/search/song?format=json&keyword=" + searcher + "&page=1&pagesize=2&showtype=1")
  if err != nil {
    return "", err
  }
  defer resp.Body.Close()

  var songInfo struct {
    Data struct {
      Info []struct {
        Hash string `json:"hash"`
      } `json:"info"`
    } `json:"data"`
  }
  err = json.NewDecoder(resp.Body).Decode(&songInfo)
  if err != nil {
    return "", err
  }

  if len(songInfo.Data.Info) == 0 {
    return "", nil
  }

  songHash := songInfo.Data.Info[0].Hash

  resp, err = client.Get("https://krcs.kugou.com/search?ver=1&man=yes&client=mobi&keyword=&duration=&hash=" + songHash + "&album_audio_id=")
  if err != nil {
    return "", err
  }
  defer resp.Body.Close()

  var lyricsInfo struct {
    Candidates []struct {
      ID        string `json:"id"`
      AccessKey string `json:"accesskey"`
    } `json:"candidates"`
  }
  err = json.NewDecoder(resp.Body).Decode(&lyricsInfo)
  if err != nil {
    return "", err
  }

  if len(lyricsInfo.Candidates) == 0 {
    return "", nil
  }

  lyricsId := lyricsInfo.Candidates[0].ID
  lyricsKey := lyricsInfo.Candidates[0].AccessKey

  resp, err = client.Get("https://lyrics.kugou.com/download?ver=1&client=pc&id=" + lyricsId + "&accesskey=" + lyricsKey + "&fmt=lrc&charset=utf8")
  if err != nil {
    return "", err
  }
  defer resp.Body.Close()

  var lyricsResponse struct {
    Content string `json:"content"`
  }
  err = json.NewDecoder(resp.Body).Decode(&lyricsResponse)
  if err != nil {
    return "", err
  }

  lrcText, err := base64.StdEncoding.DecodeString(lyricsResponse.Content)
  if err != nil {
    return "", err
  }

  lyricsText := string(lrcText)
  return lyricsText, nil
}

func saveLyricsToLrc(artist, title, lyrics string) {
  if title == "" {
    return
  }

  filename := ""
  if artist != "" {
    filename = filepath.Join(config.LyricsDir, artist+"-"+title+".lrc")
  } else {
    filename = filepath.Join(config.LyricsDir, title+".lrc")
  }

  file, err := os.Create(filename)
  if err != nil {
    fmt.Printf("Failed to create lrc file: %v\n", err)
    return
  }
  defer file.Close()

  _, err = file.WriteString(lyrics)
  if err != nil {
    fmt.Printf("Failed to write lrc content: %v\n", err)
    return
  }
}

func convertEncoding(content string) (string, error) {
  // 判断文本是否为UTF-8编码
  isUTF8 := utf8.ValidString(content)

  if !isUTF8 {
    // 将非UTF-8编码转换为UTF-8编码
    srcCharDecoder := simplifiedchinese.GBK.NewDecoder()
    srcBytes := []byte(content)
    dstReader := transform.NewReader(bytes.NewReader(srcBytes), srcCharDecoder)
    dstBytes, err := ioutil.ReadAll(dstReader)
    if err != nil {
      return "", err
    }

    // 将字节转换为UTF-8编码的字符串
    dstText := string(dstBytes)
    return dstText, nil
  } else {
    // 如果文本是UTF-8编码，则直接返回原文本
    return content, nil
  }
}

func createHTTPClient() *http.Client {
  return &http.Client{
    Transport: &http.Transport{
      TLSClientConfig: &tls.Config{
        InsecureSkipVerify: false,
      },
    },
  }
}

func initConfig() Config {
  config := Config{
    AuthEnabled:  false,
    DynamicToken: false,
    LyricsDir:    "", // default: /lyrics
    Port:         0,  // default: 25775
    RedisAddress: "", // default: localhost:6379
    Token:        "",
    Version:      "1.0.0",
    VersionFlag:  false,
  }

  flag.BoolVar(&config.AuthEnabled, "enable-auth", config.AuthEnabled, "Enable authentication")
  flag.BoolVar(&config.DynamicToken, "dynamic-token", config.DynamicToken, "Use dynamic token")
  flag.StringVar(&config.LyricsDir, "lyrics-dir", config.LyricsDir, "Specify an alternate directory for lyrics")
  flag.IntVar(&config.Port, "port", config.Port, "Server port")
  flag.StringVar(&config.RedisAddress, "redis-address", config.RedisAddress, "Redis Address")
  flag.StringVar(&config.Token, "token", config.Token, "Set a fixed token")
  flag.BoolVar(&config.VersionFlag, "version", config.VersionFlag, "Show the Lyrics API version information")

  flag.Parse()

  if !config.AuthEnabled {
    authEnabledEnv := os.Getenv("ENABLE_AUTH")
    if authEnabledEnv != "" {
      config.AuthEnabled, _ = strconv.ParseBool(authEnabledEnv)
    }
  }

  if !config.DynamicToken {
    dynamicTokenEnv := os.Getenv("DYNAMIC_TOKEN")
    if dynamicTokenEnv != "" {
      config.DynamicToken, _ = strconv.ParseBool(dynamicTokenEnv)
    }
  }

  if config.LyricsDir == "" {
    lyricsDirEnv := os.Getenv("LYRICS_DIR")
    if lyricsDirEnv != "" {
      config.LyricsDir = lyricsDirEnv
    } else {
      config.LyricsDir = "/lyrics"
    }
  }

  if config.Port == 0 {
    portEnv := os.Getenv("PORT")
    if portEnv != "" {
      config.Port, _ = strconv.Atoi(portEnv)
    } else {
      config.Port = 25775
    }
  }

  if config.RedisAddress == "" {
    redisAddressEnv := os.Getenv("REDIS_ADDRESS")
    if redisAddressEnv != "" {
      config.RedisAddress = redisAddressEnv
    } else {
      config.RedisAddress = "localhost:6379"
    }
  }

  if config.Token == "" {
    tokenEnv := os.Getenv("TOKEN")
    if tokenEnv != "" {
      config.Token = tokenEnv
    }
  }

  if config.VersionFlag {
    fmt.Printf("Lyrics API version v%s\n", config.Version)
    os.Exit(0)
  }

  return config
}
