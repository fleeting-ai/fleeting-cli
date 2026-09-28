FROM postgres:16-alpine

# Optional drop-in for the bus store. Fleeting defaults to SQLite
# (~/.fleeting/bus.db) unless FLEETING_DB or bus.db is a postgres URL.
ENV POSTGRES_USER=fleeting
ENV POSTGRES_PASSWORD=fleeting
ENV POSTGRES_DB=fleeting

EXPOSE 5432

HEALTHCHECK --interval=10s --timeout=5s --retries=10 \
  CMD pg_isready -U fleeting -d fleeting
