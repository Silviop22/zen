# ⚖️ Zen Load Balancer

A DIY load balancer which I built just for fun and to help me learn **GO** a bit better.

PS: If you are asking yourself why **GO** and not Rust for this? Because why **Rust🦀**
## ✨ Features

- **🎯 Round-Robin Load Balancing** - Evenly distributes requests across healthy backends
- **🔄 Retry Logic** - Automatically retries failed requests on different backends
- **🏊‍♂️ Connection Pooling** - Because this is a TCP load balancer
- **🩺 Health Checking** - Automatic detection and recovery of failed backends on an interval
- **⚡ High Performance I think** - Handles 2000+ req/sec with low latency overhead
- **🐳 Docker Ready** - Easy containerized deployment (Claude did it)

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- Docker (optional)

### Running Locally

1. **Clone and build:**
   ```bash
   git clone <your-repo>
   cd zen
   go mod download
   go build -o zen-lb .
   ```
   
### Docker Deployment

1. **Build the image:**
   ```bash
   docker build -t zen-load-balancer .
   ```

2. **Run with Docker:**
   ```bash
   docker run -d -p 8080:8080 -v $(pwd)/config.yaml:/root/config.yaml zen-load-balancer
   ```

## ⚙️ Configuration

### Basic Configuration

```yaml
server:
  port: 8080                    # Load balancer listening port

upstream:                       # Backend servers
  - "10.0.1.10:8080"
  - "10.0.1.11:8080"  
  - "10.0.1.12:8080"

health_check:
  enabled: true                 # Enable/disable health checking
  interval: 30s                 # How often to check backend health
  timeout: 5s                   # Timeout for each health check
  healthy_threshold: 2          # Consecutive successes to mark healthy
  unhealthy_threshold: 3        # Consecutive failures to mark unhealthy
```

### Adding/Removing Backends

To add new backends, simply update the `upstream` section in `config.yaml`:

```yaml
upstream:
  - "backend1.company.com:8080"
  - "backend2.company.com:8080"
  - "backend3.company.com:8080"
  - "backend4.company.com:8080"  # ← New backend
  - "backend5.company.com:9000"  # ← Different port
```

Then restart the load balancer:
```bash
# If running locally
./zen-lb -config config.yaml

# If running in Docker
docker restart zen-lb
```

## 🔄 Retry Mechanism

The load balancer implements retry logic to ensure high availability:

### How It Works
1. **Request arrives** → Try first backend selected by round-robin
2. **Backend fails** → Automatically retry with next available backend
3. **Max retries reached** → Return 503 error to client

### Configuration
The retry mechanism is built-in with these defaults:
- **Max retries:** 3 attempts per request
- **Retry delay:** 10ms between attempts
- **Connect timeout:** 2 seconds per backend attempt
- **Total request timeout:** 10 seconds

This part is hard coded for simplicity, normally an enterprise ready solution would offer more customization such as buffer size and so on.

### Retry Scenarios
- ✅ **Connection refused** (backend down)
- ✅ **Connection timeout** (backend overloaded)
- ✅ **DNS resolution failure**
- ✅ **Network unreachable**

### Example Retry Flow
```
Request → Backend1 (fails) → Backend2 (fails) → Backend3 (success) → Response
         ↳ 10ms delay    ↳ 10ms delay
```

## 🏊‍♂️ Connection Pooling

Zen Load Balancer uses connection pooling for optimal performance, plus TCP bro there is no abstraction here:

### Pool Configuration
Each backend maintains its own connection pool with:
- **Max idle connections:** 10 per backend
- **Max active connections:** 100 per backend
- **Idle timeout:** 30 seconds
- **Connect timeout:** 5 seconds

This was hardcoded intentionally.

### How It Works
1. **Connection reuse:** Existing connections are reused when possible
2. **Automatic cleanup:** Idle connections are closed after timeout
3. **Pool limits:** Prevents connection exhaustion
4. **Health monitoring:** Failed connections are removed from pool

### Benefits
- 🚀 **Reduced latency:** No connection setup overhead
- 💾 **Memory efficient:** Automatic cleanup of unused connections
- 🛡️ **Protection:** Prevents overwhelming backends with connections
- 📈 **Higher throughput:** More requests per second

## 🩺 Health Checking

Continuous monitoring ensures only healthy backends receive traffic:

### Health Check Process
1. **TCP connection test** to each backend every 30 seconds
2. **Consecutive failure tracking** - marks backend unhealthy after 3 failures
3. **Automatic recovery** - marks backend healthy after 2 successes
4. **Request routing** - unhealthy backends are excluded from load balancing

A next improvement would be to add a hybrid approach. You do interval checking but also mark backends as down on each retry round.
### Configuration Options

```yaml
health_check:
  enabled: true                 # Turn health checking on/off
  interval: 30s                 # Check frequency (10s-300s recommended)
  timeout: 5s                   # Individual check timeout
  healthy_threshold: 2          # Successes needed for recovery
  unhealthy_threshold: 3        # Failures needed to mark unhealthy
```

### Health Check States
- 🟢 **Healthy:** Backend receiving traffic
- 🔴 **Unhealthy:** Removed from rotation, no traffic

We only have two states to mimic the traffic lights in Albania, you either GO or you don't.

## 📊 Performance Benchmark
### Test Configuration
| Parameter | Value |
|-----------|-------|
| **Threads** | 12 |
| **Connections** | 400 |
| **Duration** | 30 seconds |
| **Target** | http://localhost:8080/ |

### Performance Results
| Metric | Value |
|--------|-------|
| **Total Requests** | 66,622 |
| **Requests/sec** | 2,215.91 |
| **Transfer/sec** | 624.49KB |
| **Total Data** | 18.34MB |
| **Test Duration** | 30.07s |

### Latency Statistics
| Statistic | Value |
|-----------|-------|
| **Average** | 177.84ms |
| **Std Deviation** | 72.43ms |
| **Maximum** | 390.75ms |
| **Distribution** | 57.81% within ±1 std dev |

### Thread Performance
| Statistic | Value |
|-----------|-------|
| **Avg Req/Sec** | 185.97 |
| **Std Deviation** | 32.67 |
| **Maximum** | 292.00 |
| **Distribution** | 66.48% within ±1 std dev |

### 🎯 Key Takeaways
- ✅ **Zero failed requests** - 100% success rate
- ⚡ **2,215 req/sec** with realistic backend latency (177ms avg)
- 🔄 **Stable performance** - consistent throughout 30-second test
- 💪 **High concurrency** - handled 400 concurrent connections smoothly

### Load Testing
```bash
# Install wrk (recommended)
brew install wrk  

# Basic load test
wrk -t12 -c100 -d30s http://localhost:8080/

# Stress test
wrk -t12 -c400 -d60s http://localhost:8080/
```

## 🐳 Docker Compose Example

For easy development and testing:

```yaml
version: '3.8'
services:
  zen-lb:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/root/config.yaml
    depends_on:
      - backend1
      - backend2
      - backend3

  backend1:
    image: nginx:alpine
    expose:
      - "80"
    
  backend2:
    image: nginx:alpine  
    expose:
      - "80"
      
  backend3:
    image: nginx:alpine
    expose:
      - "80"
```

## 🔧 Troubleshooting

### Common Issues

**Backend not receiving requests:**
```bash
# Check if backend is healthy
docker logs zen-lb | grep "backend.example.com"

# Verify backend is accessible
curl -v http://backend.example.com:8080
```

**High latency:**
```bash
# Check connection pool metrics
docker logs zen-lb | grep "connection pool"

# Monitor backend response times
docker logs zen-lb | grep "Backend work"
```

**Failed requests:**
```bash
# View error details
docker logs zen-lb | grep "ERROR"

# Check retry attempts
docker logs zen-lb | grep "Attempt"
```

### Debug Mode
Enable debug logging:
```bash
DEBUG=1 ./zen-lb -config config.yaml
```

## 📈 Monitoring

### Key Metrics to Monitor
- **Request rate:** Requests per second
- **Error rate:** Failed requests percentage
- **Backend health:** Number of healthy vs total backends
- **Response latency:** Average request duration
- **Connection pool usage:** Active vs idle connections

### Log Analysis
```bash
# Request distribution across backends
docker logs zen-lb | grep "backend server" | sort | uniq -c

# Health check failures
docker logs zen-lb | grep "Health check FAILED"

# Performance metrics
docker logs zen-lb | grep "Backend pool updated"
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

I have a driving licence, if that is what you are asking.

## 🔭 Whats next
 - Log Rolling
 - Configurable balancing algorithm (Round Robin, IP Hash, Weighted approach, etc.)
---

**Built with ❤️, Claude and GO**