1. Install RabbitMQ and make sure it's running
2. Create a .env file based on .env example -
```
GITHUB_TOKEN=...
RABBITMQ_SERVER_URL=amqp://guest:guest@localhost:5672/
```
3. Run the service (I provided an Air config to ease that process)
```$ air```
4. '/scan' requests begin scanning, for example:
```/scan?author=ronen25&repo=nautilus-copypath```
5. '/repo' retrieves the results if they're ready (results are cached in-memory but in the future can be serialized to a file as well):
```/repo?author=ronen25&repo=nautilus-copypath```