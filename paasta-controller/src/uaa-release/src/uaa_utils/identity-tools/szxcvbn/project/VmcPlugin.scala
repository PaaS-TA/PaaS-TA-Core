import sbt._
import sbt.Keys._

// VMC Packaging, based on ideas from the outdated package-dist plugin
object VmcPlugin {
  val vmcPackage = TaskKey[File]("vmc-package", "package a standalone server zip file for 'vmc push'")
  val vmcPackageDir = SettingKey[File]("vmc-package-dir", "the directory to create the vmc package in")
  val vmcPackageZipName = SettingKey[String]("vmc-package-zip-name", "name of vmc package zip file")
//  val vmcPackageClean = TaskKey[Unit]("vmc-package-clean", "delete the zip file and intermediate directories")
  val vmcPackageCopyJars =
    TaskKey[Set[File]]("vmc-package-copy-jars", "copy exported files into the zip folder")
  val vmcPackageCopyLibs =
    TaskKey[Set[File]]("vmc-package-copy-libs", "copy library dependencies into the zip folder")
  val vmcPackageCopy =
    TaskKey[Set[File]]("vmc-package-copy", "copy all dist files into the zip folder")
  val vmcPackageZipPath =
    TaskKey[String]("vmc-package-zip-path", "path of files inside the packaged zip file")

  val vmcSettings = Seq(
    exportJars := true,

    vmcPackageDir <<= target.apply(_ / "vmc-archive"),

    packageOptions <+= (dependencyClasspath in Compile, mainClass in Compile) map { (cp, main) =>
      val manifestClasspath = cp.files.map(f => "libs/" + f.getName).mkString(" ")
      val attrs = Seq(("Class-Path", manifestClasspath)) ++ main.map { ("Main-Class", _) }
      Package.ManifestAttributes(attrs: _*)
    },

    vmcPackageCopyJars <<= (
      exportedProducts in Compile,
      vmcPackageDir
      ) map { (products, dest) =>
      IO.copy((products).files.map(p => (p, dest / p.getName)))
    },

    vmcPackageCopyLibs <<= (
      dependencyClasspath in Runtime,
      exportedProducts in Compile,
      vmcPackageDir
      ) map { (cp, products, dest) =>
      val jarFiles = cp.files.filter(f => !products.files.contains(f))
      val jarDest = dest / "libs"
      jarDest.mkdirs()
      IO.copy(jarFiles.map { f => (f, jarDest / f.getName) })
    },

    vmcPackageCopy <<= (
      vmcPackageCopyLibs,
      vmcPackageCopyJars
      ) map { (libs, jars) => libs ++ jars },

    vmcPackageZipName <<= normalizedName.apply("%s.zip".format(_)),

    vmcPackageZipPath := "",

    vmcPackage <<= (
      baseDirectory,
      vmcPackageCopy,
      vmcPackageDir,
      vmcPackageZipPath,
      vmcPackageZipName,
      streams
      ) map { (base, files, dest, zipPath, zipName, s) =>
      s.log.info("Building %s from %d files.".format(zipName, files.size))
      val zipRebaser = Path.rebase(dest, zipPath)
      val zipFile = base / zipName
      IO.zip(files.map(f => (f, zipRebaser(f).get)), zipFile)
      zipFile
    }
  )
}
