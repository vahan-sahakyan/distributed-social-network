# Distributed Social Network — Local System Design POC

## Project Goal

Build a **fully local, production-style distributed social network backend** to demonstrate advanced system design concepts.

The entire system must run **without any cloud provider dependencies**.

Primary purpose:

* Learn distributed systems
* Practice microservice architecture
* Simulate production infrastructure locally
* Build a portfolio-level backend project

Architecture principles:

* Microservices
* CQRS
* Event Driven Architecture
* Eventual Consistency
* Database Isolation
* Horizontal Scalability
* Async Communication
* Service Decoupling

Deployment target:

* Fully local
* Docker Compose support
* Kubernetes support (k3d / Minikube)

---

# Functional Requirements

The platform must support:

### Posts

Users can:

* Create posts
* Attach image to post
* View posts

### Feed

Users can:

* View home feed
* View another user's feed

### Social Interaction

Users can:

* Like posts
* Like images
* Comment on posts
* Comment on images
* Read comments

### Media

Users can:

* Upload images
* Retrieve images

### Users

Users can:

* Have profiles
* Follow other users
* Maintain social graph

---

# Non Functional Requirements

Availability target:

* 99.99%

Consistency model:

* Eventual consistency

Latency target:

* p95 < 3000ms

Architecture style:

* Distributed microservices

Scalability:

* Horizontal scaling supported

Infrastructure:

* Entire system must run locally

---

# Scale Assumptions

Daily active users (simulation):

50,000,000 DAU

Per user daily behavior:

* 1 post/day
* Open home feed 20 times/day
* Open user feed 5 times/day
* Create 10 likes/day
* Create 2 comments/day
* Read comments 10 times/day

Estimated throughput:

| Action          | RPS  |
| --------------- | ---- |
| Create post     | 1.8k |
| Home feed reads | 36k  |
| User feed reads | 9k   |
| Likes           | 18k  |
| Comments        | 3.6k |
| Read comments   | 18k  |

Read/write ratio:

2.7 : 1

---

# Storage Estimation

Posts:

25 TB/year

Comments:

12 TB/year

Images:

1.2 PB/year

Optimizations:

* Image compression
* CDN caching simulation
* Thumbnail generation

---

# Domain Entities

## Post

```go
type Post struct {
    ID string

    Text string

    AuthorID string

    ImageID string

    Likes int

    TopLikes []Like

    Comments int

    TopComment Comment

    CreatedAt time.Time
}
```

---

## Like

```go
type Like struct {
    ID string

    UserID string

    EntityID string
}
```

EntityID may reference:

* Post
* Comment
* Image

---

## Comment

```go
type Comment struct {
    ID string

    UserID string

    EntityID string

    Text string

    Likes int

    CreatedAt time.Time
}
```

EntityID may reference:

* Post
* Image

---

## Image

```go
type Image struct {
    ID string

    Likes int

    Comments int

    TopLikes []Like
}
```

---

## HomeFeed

```go
type HomeFeed struct {
    Posts []Post
}
```

---

## UserFeed

```go
type UserFeed struct {
    Posts []Post
}
```

---

# API Design

## Posts API

Create post

```http
POST /posts
```

Request:

```json
Post
```

---

## Feed API

Get home feed

```http
GET /feed/home
```

Response:

```json
HomeFeed
```

Get user feed

```http
GET /feed/user/{user_id}
```

Response:

```json
UserFeed
```

---

## Likes API

Create like

```http
POST /likes
```

Request:

```json
Like
```

---

## Comments API

Create comment

```http
POST /comments
```

Request:

```json
Comment
```

Get comments

```http
GET /posts/{post_id}/comments
```

Response:

```json
Comment[]
```

---

## Media API

Upload image

```http
POST /images/upload
```

Request:

```json
byte[]
```

Response:

```json
Image
```

---

# Service Architecture

System consists of independent microservices.

```text
gateway-service

posts-service

comments-service

likes-service

feed-service

users-service

media-service

notification-service
```

---

# CQRS Architecture

Separate write and read responsibilities.

## Write Operations

```text
POST /posts

POST /likes

POST /comments

POST /images/upload
```

Each service writes to its own database.

Then publishes events.

---

## Read Operations

```text
GET /feed/home

GET /feed/user/{id}

GET /posts/{id}/comments
```

Dedicated read model.

Read path optimized separately.

---

# Event Driven Architecture

Communication must be asynchronous.

Message broker:

```text
Redpanda or Kafka
```

Services communicate through events.

---

# Event Topics

```text
post.created

comment.created

like.created

image.uploaded

feed.updated

notification.created
```

---

# Infrastructure Stack (Local Only)

No cloud services allowed.

---

## Object Storage

Use:

MinIO

Purpose:

* Store images
* S3 compatible API

Replaces:

* AWS S3

---

## Message Broker

Use one:

Option 1:

Redpanda

Option 2:

Apache Kafka

Purpose:

* Async communication

Replaces:

* AWS MSK

---

## Databases

Use PostgreSQL.

Separate database per service.

```text
posts-db

comments-db

likes-db

feed-db

users-db
```

Replaces:

* AWS RDS

---

## Cache Layer

Use Redis.

Purpose:

* Feed caching
* Hot posts caching
* Counters
* Sessions

Replaces:

* ElastiCache

---

## API Gateway

Use one:

* Traefik
* Nginx

Purpose:

* Routing
* Load balancing

Replaces:

* AWS ELB

---

# Database Isolation

Every service owns its own database.

No shared databases allowed.

---

## Posts Database

```text
posts-db
```

Contains:

```sql
posts
```

Sharding strategy:

```text
author_id % shard_count
```

---

## Comments Database

```text
comments-db
```

Contains:

```sql
comments
```

Sharding strategy:

```text
entity_id % shard_count
```

---

## Likes Database

```text
likes-db
```

Contains:

```sql
likes
```

Sharding strategy:

```text
entity_id % shard_count
```

---

# Feed Generation Strategy

Use:

Fanout on Write

Flow:

```text
Post Created

↓

posts-service stores post

↓

publish event post.created

↓

feed-service receives event

↓

find followers

↓

create feed entries

↓

store denormalized feed
```

---

# Feed Read Model

Dedicated database optimized for reads.

Table:

```sql
feed_items
```

Schema example:

```sql
CREATE TABLE feed_items (
    user_id TEXT,
    post_id TEXT,
    author_id TEXT,
    text TEXT,
    likes_count INTEGER,
    comments_count INTEGER,
    image_url TEXT,
    created_at TIMESTAMP
);
```

Purpose:

Fast reads.

---

# Media Service

Responsible for:

* Image upload
* Metadata storage

Storage backend:

MinIO

Pipeline:

```text
Upload image

↓

Store in MinIO

↓

Generate metadata

↓

Publish image.uploaded event
```

Future:

* Compression
* Thumbnail generation

---

# Notification Service

Consumes events:

```text
like.created

comment.created
```

Creates notifications.

Future support:

* WebSockets
* Push notifications

---

# Monitoring Stack

Use:

Prometheus

Purpose:

* Metrics collection

Grafana

Purpose:

* Visualization

Loki

Purpose:

* Log aggregation

Jaeger

Purpose:

* Distributed tracing

---

# Deployment

Must support:

---

## Docker Compose

For quick local startup.

```text
docker-compose up
```

---

## Kubernetes

Local cluster only.

Options:

* k3d
* Minikube

Deployment via:

* Helm

Optional:

* ArgoCD

---

# Local Infrastructure Layout

```text
distributed-social-network/

services/

  gateway-service/

  posts-service/

  comments-service/

  likes-service/

  feed-service/

  users-service/

  media-service/

  notification-service/

infrastructure/

  docker-compose.yml

deploy/

  kubernetes/

monitoring/

  prometheus/

  grafana/

  loki/

docs/

  architecture.md
```

---

# Future Improvements

Infrastructure:

* Service discovery
* Chaos testing
* Horizontal autoscaling

Features:

* Search service
* Recommendation engine
* Video uploads
* Stories
* Real-time feed updates

Performance:

* Redis clustering
* DB replication
* Read replicas
* Multi-region simulation

Security:

* Rate limiting
* Authentication service
* API keys
* JWT
* RBAC

---

# Technology Stack

Language:

Go

Framework:

Gin or Fiber

Databases:

PostgreSQL

Messaging:

Kafka or Redpanda

Cache:

Redis

Object Storage:

MinIO

Gateway:

Traefik or Nginx

Containerization:

Docker

Orchestration:

Kubernetes

Observability:

Prometheus
Grafana
Loki
Jaeger

CI/CD:

GitHub Actions
ArgoCD

---

# Project Objective

This project must resemble production-grade distributed architecture while running entirely on local infrastructure.

Purpose:

* Learn system design
* Practice distributed systems engineering
* Demonstrate backend engineering skills
* Demonstrate DevOps and Kubernetes knowledge
* Build a senior-level portfolio project

```
```
