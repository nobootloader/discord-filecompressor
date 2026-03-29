> [!NOTE]
> This project is in a work in progress state

# Overview
The goal of this website is to provide an easy way to compress video files to the Discord free-tier size limit of 10 MB. The backend is implemented in a microservice architecture and hosted on Cloudflare using Amazon S3 SDKs. 
There was necessarily no need to implement this kind of architecture, nor was it the most simple or smartest way to do this, but it was done for the sake of learning and improving as a software developer. The same functionality could have attained with AWS Lambda functions.

# Technologies
**Backend:** Go

**Observability:** Open Telemetry, Jaeger

**Server:** Docker, AWS S3 (SDK, using R2 bucket), Kubernetes, Kafka 

**Frontend:** TypeScript, Vue.js

## Requirements
```sh
go 1.24 or newer
```
