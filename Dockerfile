FROM alpine:3.8
LABEL Name=hub-grafana-agent \
      Release=https://github.com/appvia/hub-grafana-agent \
      Maintainer=danielwhatmuff@gmail.com \
      Url=https://github.com/appvia/hub-grafana-agent \
      Help=https://github.com/appvia/hub-grafana-agent/issues

RUN apk add --no-cache ca-certificates curl

ADD bin/hub-grafana-agent /hub-grafana-agent

USER 65534

ENTRYPOINT [ "/hub-grafana-agent" ]
