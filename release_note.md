# wehook

```
  -cert="cert.pem": path to the HTTPS certificate pem file
  -hooks="hooks.json": path to the json file containing defined hooks the webhook should serve
  -hotreload=false: watch hooks file for changes and reload them automatically
  -ip="": ip the webhook should serve hooks on
  -key="key.pem": path to the HTTPS certificate private key pem file
  -port=9000: port the webhook should serve hooks on
  -secure=false: use HTTPS instead of HTTP
  -urlprefix="hooks": url prefix to use for served hooks (protocol://yourserver:port/PREFIX/:hook-id)
  -verbose=false: show verbose output
```