FROM openjdk:8-jdk-alpine
VOLUME /tmp
ADD go-fastdfs-web.jar app.jar
RUN bash -c 'touch /app.jar'
ENTRYPOINT ["java","-Djava.security.egd=file:/dev/./urandom","-jar","/app.jar"]