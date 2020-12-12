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

//
// Bundles Golang Lambda function from source
export function AssetCodeGo(path: string): lambda.Code {
  return new lambda.AssetCode('', { bundling: gocc(path) })
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


const gocc = (path: string): cdk.BundlingOptions => {
  const gopath = process.env.GOPATH || '/go'

  const workingDirectory = path.startsWith(gopath)
    ? `/go${path.split(gopath).join('')}`
    : path

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
