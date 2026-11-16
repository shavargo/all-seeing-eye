# OpenMaster - Web Directory & Connection Hub

Ultra-lightweight static web directory and reverse proxy powered by `nginx:alpine`.

## Overview
- Serves the OpenMaster server directory and client connection guides.
- Proxies `/api/` requests directly to the master server backend on port `8085`.
- Instant sub-second container startup with zero Node.js overhead.
- Total image size is ~30 MB.

## Development / Local Preview
Preview the directory by opening `public/index.html` in any browser, or running Nginx:

```bash
docker build -f Dockerfile.web -t openmaster-web .
docker run -p 80:80 openmaster-web
```
