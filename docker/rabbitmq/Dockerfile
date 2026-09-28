FROM rabbitmq:3.13-management-alpine

# Guest on localhost matches examples/fleet.yaml. Change these for anything
# that is not a local lab box.
ENV RABBITMQ_DEFAULT_USER=guest
ENV RABBITMQ_DEFAULT_PASS=guest

EXPOSE 5672 15672

HEALTHCHECK --interval=10s --timeout=5s --retries=10 \
  CMD rabbitmq-diagnostics -q ping
