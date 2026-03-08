plugins {
    java
    id("com.github.johnrengelman.shadow") version "8.1.1"
}

group = "com.minecraft"
version = "1.0.0"

java {
    toolchain {
        languageVersion.set(JavaLanguageVersion.of(21))
    }
}

repositories {
    mavenCentral()
    maven("https://repo.papermc.io/repository/maven-public/")
    maven("https://repo.opencollab.dev/maven-snapshots/")
    maven("https://repo.opencollab.dev/maven-releases/")
}

dependencies {
    compileOnly("com.velocitypowered:velocity-api:3.4.0-SNAPSHOT")
    annotationProcessor("com.velocitypowered:velocity-api:3.4.0-SNAPSHOT")

    // Floodgate API for Bedrock player detection
    compileOnly("org.geysermc.floodgate:api:2.2.2-SNAPSHOT")

    // JSON handling
    implementation("com.google.code.gson:gson:2.10.1")
}

tasks {
    build {
        dependsOn(shadowJar)
    }

    shadowJar {
        archiveBaseName.set("WhitelistRouter")
        archiveClassifier.set("")
    }
}
