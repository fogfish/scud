//
// Copyright (C) 2020 Dmitry Kolesnikov
//
// This file may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
// https://github.com/fogfish/scud
//
import * as cdk from '@aws-cdk/core'
import * as lambda from '@aws-cdk/aws-lambda'
import * as sys from 'child_process'
import * as path from 'path'
import * as crypto from 'crypto'
import * as fs from 'fs'

/**
 * Bundles Golang Lambda function from source
 *
 * @param sourceCodePackage - absolute path to source code of serverless app (e.g. $GOPATH/src/github.com/fogfish/scud)
 * @param sourceCodeLambda  - relative path to lambda function, the path to main package
 */
export function AssetCodeGo(sourceCodePackage: string, sourceCodeLambda: string): lambda.Code {
  return new lambda.AssetCode('', {
    bundling: gocc([sourceCodePackage, sourceCodeLambda].join(path.sep)),
    assetHashType: cdk.AssetHashType.CUSTOM,
    assetHash: hash(sourceCodePackage),
  })
}

const walk = (dirname: string): string[] => {
  const goFiles = new RegExp('(.*\.go$)|(.*\.(mod|sum)$)')
  const dirents = fs.readdirSync(dirname, { withFileTypes: true })
  const files = dirents
    .filter(dirent => dirent.isFile())
    .filter(dirent => goFiles.test(dirent.name))
    .map(dirent => path.join(dirname, dirent.name))
  
  const subdirs = dirents
    .filter(dirent => dirent.isDirectory())
    .map(dirent => walk(path.join(dirname, dirent.name)))
  
  return files.concat(...subdirs)
}

const hash = (source: string): string => {
  const sha = crypto.createHash('sha256')

  walk(source).forEach(file => {
    const data = fs.readFileSync(file)
    sha.update(`<file name=${file}>`)
    sha.update(data)
    sha.update('</file>')
  })
  const codeHash = sha.digest('hex')
  // tslint:disable-next-line:no-console
  console.log(`==> ${source} ${codeHash}`)
  return codeHash
}

const tryBundle = (outputDir: string, options: cdk.BundlingOptions): boolean => {
  if (!options || !options.workingDirectory) {
    return false
  }

  const pkg = options.workingDirectory.split('/go/src/').join('')
  // tslint:disable-next-line:no-console
  console.log(`==> go build ${pkg}`)
  sys.execSync(`GOCACHE=/tmp/go.amd64 GOOS=linux GOARCH=amd64 go build -o ${outputDir}/main ${pkg}`)
  return true
}

const gocc = (codepath: string): cdk.BundlingOptions => {
  const gopath = process.env.GOPATH || '/go'

  const workingDirectory = codepath.startsWith(gopath)
    ? `/go${codepath.split(gopath).join('')}`
    : codepath

  return {
    local: { tryBundle },
    image: cdk.BundlingDockerImage.fromRegistry('golang'),
    command: ["go", "build", "-o", `${cdk.AssetStaging.BUNDLING_OUTPUT_DIR}/main`],
    user: 'root',
    environment: {
      "GOCACHE": "/go/cache",
    },
    volumes: [
      {
        containerPath: '/go/src',
        hostPath: `${gopath}/src`,
      },
    ],
    workingDirectory,
  }
}
