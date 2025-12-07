# Leaderboard System - Claude Code Skills

This directory contains specialized skills for working with the real-time gaming leaderboard system. These skills are based on the system design principles from the project documentation.

## Available Skills

### 🚀 deploy-leaderboard
**Purpose**: Deploy the complete leaderboard system to K3d cluster

**When to use**:
- Initial setup of the system
- Clean deployment after changes
- Setting up demo environment

**What it does**:
- Creates K3d cluster
- Deploys PostgreSQL, Valkey, and the Go application
- Sets up monitoring (Prometheus + Grafana)
- Provides access URLs and verification steps

**Example usage**:
```
You: Deploy the leaderboard system
```

---

### 🧪 run-performance-test
**Purpose**: Execute and analyze performance tests using K6

**When to use**:
- After deployment to verify performance
- Comparing PostgreSQL vs Redis performance
- Load testing before production
- Benchmarking after optimizations

**What it does**:
- Runs K6 test scenarios
- Analyzes response times and throughput
- Generates comparison reports
- Guides you through Grafana metrics

**Example usage**:
```
You: Run performance tests and compare both scenarios
```

---

### 🏗️ analyze-architecture
**Purpose**: Deep dive into system design decisions and architecture

**When to use**:
- Understanding design choices
- System design interview preparation
- Code review and architectural assessment
- Planning improvements or modifications

**What it does**:
- Explains key design patterns (sorted sets, scatter-gather, etc.)
- Analyzes trade-offs and alternatives
- Reviews current implementation
- Suggests optimizations

**Example usage**:
```
You: Explain why we use Redis sorted sets instead of PostgreSQL
You: Analyze the current architecture and suggest improvements
```

---

### 🔧 troubleshoot-leaderboard
**Purpose**: Debug and fix issues in the deployed system

**When to use**:
- Pods are failing or crashing
- Performance issues
- Connection problems
- Deployment errors

**What it does**:
- Provides systematic debugging workflow
- Common issues and solutions
- Diagnostic commands
- Step-by-step troubleshooting guide

**Example usage**:
```
You: The app pods are in CrashLoopBackOff, help me debug
You: Performance is slow, what should I check?
```

---

### 📈 scale-leaderboard
**Purpose**: Scale the system from 5M to 500M+ users

**When to use**:
- Planning for growth (10x, 100x scale)
- Implementing sharding strategies
- Optimizing for higher loads
- Cost optimization at scale

**What it does**:
- Explains vertical vs horizontal scaling
- Implements Redis sharding (fixed partition or hash partition)
- Scatter-gather pattern for distributed queries
- Capacity planning and calculations

**Example usage**:
```
You: How do I scale this to 100 million users?
You: Implement Redis sharding with 6 shards
```

---

## How to Use Skills

### Method 1: Direct skill invocation
You can directly invoke a skill by mentioning it:
```
You: Use the deploy-leaderboard skill
```

### Method 2: Natural conversation
Claude will automatically use the appropriate skill based on your question:
```
You: Help me deploy this system
→ Uses deploy-leaderboard skill

You: Why is my pod crashing?
→ Uses troubleshoot-leaderboard skill

You: Explain the Redis sorted set design
→ Uses analyze-architecture skill
```

## Skill Workflow Examples

### Complete Setup Flow
1. **Deploy**: "Deploy the leaderboard system"
2. **Test**: "Run performance tests"
3. **Analyze**: "Show me the results comparison"
4. **Optimize**: "How can I improve the performance?"

### Debugging Flow
1. **Identify**: "The app won't start"
2. **Troubleshoot**: "Help me debug this issue"
3. **Fix**: Follow the diagnostic steps
4. **Verify**: "Run tests to verify it's working"

### Scaling Flow
1. **Assess**: "What's my current capacity?"
2. **Calculate**: "I need to handle 50M users"
3. **Design**: "Design a sharding strategy"
4. **Implement**: "Help me implement Redis cluster"
5. **Test**: "Run load tests at scale"

## Project Context

These skills are designed for a leaderboard system with:
- **Scale**: 5 million DAU (scalable to 500M+)
- **Performance**: <100ms response time, 2,500+ QPS
- **Tech Stack**: Go, PostgreSQL, Valkey (Redis), K3d, K8s
- **Architecture**: Microservices with Redis sorted sets

## Learning Path

If you're learning system design:

1. Start with **analyze-architecture** to understand design decisions
2. Use **deploy-leaderboard** to see it in action
3. Run **run-performance-test** to see the performance difference
4. Try **scale-leaderboard** to understand scaling strategies
5. Practice **troubleshoot-leaderboard** for operational skills

## Key Concepts Covered

- **Redis Sorted Sets**: O(log n) operations for real-time leaderboards
- **Scatter-Gather**: Distributed query pattern for sharded data
- **Server Authority**: Security pattern for authoritative actions
- **Back-of-the-envelope**: Capacity planning calculations
- **Trade-offs**: Performance vs complexity vs cost

## Documentation References

- `README.md`: High-level design overview
- `leader_board.pdf`: Complete system design interview chapter
- `Makefile`: Automation commands
- `k6/`: Performance test scenarios
- `src/`: Go application code

## Tips

1. **Start simple**: Deploy first, then experiment
2. **Use monitoring**: Grafana at http://localhost:30300
3. **Understand trade-offs**: Every decision has costs and benefits
4. **Test at scale**: Use K6 to simulate realistic load
5. **Read the code**: The implementation shows the theory in practice

## Getting Help

If you're stuck:
1. Ask for help from a specific skill: "Use troubleshoot-leaderboard to help me"
2. Provide context: Share error messages, logs, or commands you tried
3. Be specific: What were you trying to do when the issue occurred?

Happy coding! 🚀
