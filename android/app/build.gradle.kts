import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// The KC9K extension, packed from the desktop's sources (internal/webext/files) into the
// app's assets, so IronFox runs exactly the extension LibreWolf runs. Entries are sorted
// and dated 0, like webext.Pack, so the .xpi is the same on every build.
val webextDir = rootProject.projectDir.resolve("../internal/webext/files")
val xpiAssets = layout.buildDirectory.dir("generated/xpi")
val packExtension by tasks.registering {
    inputs.dir(webextDir)
    outputs.dir(xpiAssets)
    doLast {
        val out = xpiAssets.get().asFile.apply { mkdirs() }
        ZipOutputStream(out.resolve("kc9k.xpi").outputStream()).use { zip ->
            webextDir.listFiles()!!.filter { it.isFile }.sortedBy { it.name }.forEach { f ->
                zip.putNextEntry(ZipEntry(f.name).apply { time = 0 })
                zip.write(f.readBytes())
                zip.closeEntry()
            }
        }
        val version = Regex("\"version\"\\s*:\\s*\"([^\"]+)\"").find(webextDir.resolve("manifest.json").readText())!!.groupValues[1]
        out.resolve("kc9k-version.txt").writeText(version)
    }
}

android {
    namespace = "org.kc9k.android"
    compileSdk = 36
    defaultConfig {
        applicationId = "org.kc9k.android"
        minSdk = 29
        targetSdk = 36
        versionCode = 2
        versionName = "0.2"
    }
    buildFeatures { buildConfig = true }
    sourceSets["main"].assets.srcDir(xpiAssets)
    buildTypes {
        release { isMinifyEnabled = false }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }
}

tasks.named("preBuild") { dependsOn(packExtension) }

dependencies {
    testImplementation("junit:junit:4.13.2")
}
