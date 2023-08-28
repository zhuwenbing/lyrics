# lyrics
### Instructions for using Docker Compose
```yaml
services:
  lyrics:
    # 使用 --auth-enable 开启鉴权
    # 使用 --dynamic-token 启用动态token
    # 使用 --lyrics-dir=<path> 设置歌词目录
    # 使用 --port=<port> 设置歌词服务端口
    # 使用 --redis-address=<redis—address> 设置Redis服务地址
    # 使用 --token=<token> 设置固定token
    # 使用示例如下（命令行参数优先于环境变量）：
    # command: lyrics --auth-enable --token=18QqYEwM96ePFEI1 --version
    container_name: lyrics
    environment:
      AUTH_ENABLE: "true" # 是否开启鉴权，默认false
      # DYNAMIC_TOKEN: "false" # 是否启用动态token，需要搭配Redis使用，配默认false
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