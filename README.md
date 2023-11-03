# lyrics
### Instructions for using Docker Compose
```yaml
services:
  # 注意：本应用中token限定为由大小写字母及数字组成，长度为8~16的字符串
  lyrics:
    # 使用 --enable-auth 开启鉴权
    # 使用 --dynamic-token 启用动态token。从Redis中读取token，从而支持多用户鉴权以及控制有效期
    # 使用 --lyrics-dir=<path> 设置歌词目录
    # 使用 --port=<port> 设置歌词服务端口
    # 使用 --redis-address=<redis—address> 设置Redis服务地址
    # 使用 --token=<token> 设置固定token
    # 使用示例如下（命令行参数优先于环境变量）：
    # command: lyrics --enable-auth --token=18QqYEwM96ePFEI1 --version
    container_name: lyrics
    environment:
      # DYNAMIC_TOKEN: "false" # 是否启用动态token，需要搭配Redis使用，默认false
      ENABLE_AUTH: "true" # 是否开启鉴权，默认false
      # LYRICS_DIR: "/lyrics" # 歌词目录，默认“/lyrics”
      # PORT: 25775 # 歌词服务端口，默认25775
      # REDIS_ADDRESS: redis:6379 # Redis服务地址，默认localhost:6379
      TOKEN: 18QqYEwM96ePFEI1 # 固定token，仅当开启鉴权且未启用动态token时有效
    healthcheck:
      test: ["CMD", "wget", "-cqS", "--spider", "http://localhost:25775/"]
    image: kissice/lyrics
    logging:
      driver: json-file
      options:
        max-file: "3"
        max-size: 5m
    networks:
      my_bridge:
        aliases:
        - lyrics
    restart: always
    user: 1000:1000
    volumes:
    - /home/kissice/music/lyrics:/lyrics:rw

networks:
  my_bridge:
    driver: bridge
    name: my-bridge
```
