FROM alpine:3.20
RUN adduser -D app
USER app
WORKDIR /home/app
COPY dist/bot/bot /usr/local/bin/bot
ENTRYPOINT ["bot"]
