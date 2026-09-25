#!/usr/bin/env python3
"""
Simple load test for VogueFlow.
Usage: python loadtest.py --rps 2 --duration 10

WARNING: cache misses call the Claude API and cost real money. Keep RPS low.
"""
import argparse
import asyncio
import aiohttp
import time
import json
import random
import statistics

URL = "http://localhost:8080/v1/shop"

QUERIES = [
    "I need a red date night outfit under $150",
    "What size bra for 34D broad shoulders?",
    "Find me a wedding guest dress",
    "Black lingerie set in stock",
    "Stylist help for beach vacation",
]

async def send_request(session, query):
    # No "embedding": the server embeds the query text with Pinecone.
    payload = {
        "session_id": f"sess_{random.randint(1, 1000000)}",
        "user_id": f"user_{random.randint(1, 100000)}",
        "query": query,
        "budget": 150.0,
    }
    start = time.time()
    try:
        async with session.post(URL, json=payload) as resp:
            await resp.read()
            latency = (time.time() - start) * 1000
            return latency, resp.status
    except Exception as e:
        return (time.time() - start) * 1000, 0

async def worker(session, queue, results):
    while True:
        query = await queue.get()
        if query is None:
            break
        lat, status = await send_request(session, query)
        results.append((lat, status))
        queue.task_done()

async def run_load_test(target_rps, duration_sec, workers=100):
    queue = asyncio.Queue(maxsize=target_rps * 2)
    results = []
    
    async with aiohttp.ClientSession() as session:
        # Start workers
        tasks = [asyncio.create_task(worker(session, queue, results)) for _ in range(workers)]
        
        # Producer: inject requests at target RPS
        start_time = time.time()
        interval = 1.0 / target_rps
        
        while time.time() - start_time < duration_sec:
            query = random.choice(QUERIES)
            try:
                queue.put_nowait(query)
            except asyncio.QueueFull:
                pass
            await asyncio.sleep(interval)
        
        # Signal workers to stop
        for _ in range(workers):
            await queue.put(None)
        await asyncio.gather(*tasks)
    
    # Report
    latencies = [r[0] for r in results if r[1] == 200]
    errors = [r for r in results if r[1] != 200]
    
    print(f"\n=== Load Test Results ===")
    print(f"Total Requests: {len(results)}")
    print(f"Successful: {len(latencies)}")
    print(f"Errors: {len(errors)}")
    if latencies:
        print(f"Min Latency: {min(latencies):.2f}ms")
        print(f"Max Latency: {max(latencies):.2f}ms")
        print(f"Mean Latency: {statistics.mean(latencies):.2f}ms")
        print(f"p50: {statistics.median(latencies):.2f}ms")
        print(f"p99: {sorted(latencies)[int(len(latencies)*0.99)]:.2f}ms")
    print(f"Effective RPS: {len(results) / duration_sec:.1f}")

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--rps", type=int, default=2)
    parser.add_argument("--duration", type=int, default=10)
    parser.add_argument("--workers", type=int, default=4)
    args = parser.parse_args()
    
    asyncio.run(run_load_test(args.rps, args.duration, args.workers))