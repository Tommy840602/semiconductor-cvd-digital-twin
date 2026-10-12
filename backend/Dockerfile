FROM maven:3.9.9-eclipse-temurin-21 AS build
WORKDIR /src
COPY pom.xml .
COPY src src
RUN mvn -q -B package -DskipTests
FROM eclipse-temurin:21-jre-alpine
ENV JAVA_TOOL_OPTIONS="-Djava.awt.headless=true"
WORKDIR /app
COPY --from=build /src/target/cvd-0.1.0.jar /app/app.jar
ENTRYPOINT ["java","-jar","/app/app.jar"]
