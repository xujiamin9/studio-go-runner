#!/bin/bash -e

[ -z "$USER" ] && echo "Error: env variable USER must be set" && exit 1;
[ -z "$GOPATH" ] && echo "Error: env variable GOPATH must be set" && exit 1;
[ -z "$GITHUB_TOKEN" ] && echo "Warning : env variable GITHUB_TOKEN should be set in the event that a release is to be generated" ;
[ -z ${azure_registry_name+x} ] && echo "Warning : env variable azure_registry_name not set";

if [[ ":$PATH:" != *":$GOPATH/bin:"* ]]; then
    export PATH=$PATH:$GOPATH/bin
fi

export LOGXI="*=DBG"
export LOGXI_FORMAT="happy,maxcol=1024"

[ -z "$TERM" ] && export TERM=xterm+256color;

if [ -n "$(type -t travis_fold)" ] && [ "$(type -t travis_fold)" = function ]; then
    type travis_fold
    type travis_nanoseconds
    type travis_time_start
    type travis_time_finish
:
else
declare -i travis_start_time
declare -i travis_end_time

	function travis_nanoseconds () {
		local cmd="date";
		local format="+%s%N";
		local os=$(uname);
		if hash gdate > /dev/null 2>&1; then
			cmd="gdate";
		else
			if [[ "$os" = Darwin ]]; then
				format="+%s000000000";
			fi;
		fi;
		$cmd -u $format
	}

    function travis_fold() {
        local action=$1;
        local name=$2;
        echo -en "travis_fold:${action}:${name}\r${ANSI_CLEAR}"
    }
    function travis_time_start() {
        travis_timer_id=$(printf %08x $(( RANDOM * RANDOM )));
        travis_start_time=$(travis_nanoseconds);
        echo -en "travis_time:start:$travis_timer_id\r${ANSI_CLEAR}"
    }
    function travis_time_finish() {
        local result=$?;
        travis_end_time=$(travis_nanoseconds);
        local duration=$(($travis_end_time-$travis_start_time));
        echo -en "\ntravis_time:end:$travis_timer_id:start=$travis_start_time,finish=$travis_end_time,duration=$duration\r${ANSI_CLEAR}";
        return $result

    }
fi

go get github.com/karlmutch/petname

go get -u github.com/golang/dep/cmd/dep

dep ensure

bash -c "while true; do echo \$(date) - building ...; sleep 180s; done" &
PING_LOOP_PID=$!

function cleanup {
    # nicely terminate the ping output loop
    kill $PING_LOOP_PID
}
trap cleanup EXIT

function ExitWithError
{
    echo "$*">&2
    rm -f $working_file
    exit 1
}

function Tidyup
{
    ExitWithError "Abort"
}

umask 077
working_file=$$.studio-go-runner-working
rm -f $working_file
trap Tidyup 1 2 3 15

export SEMVER=`semver`
export GIT_BRANCH=`echo '{{.duat.gitBranch}}'|stencil - | tr '_' '-' | tr '\/' '-'`
GIT_COMMIT=`git rev-parse HEAD`
export RUNNER_BUILD_LOG=build-$GIT_BRANCH.log
exit_code=0

# Build the base image that other images will derive from for development style images
docker build -t leafai/studio-go-runner-dev-base:0.0.0 -f Dockerfile_base .

travis_fold start "build.image"
    travis_time_start
        stencil -input Dockerfile | docker build -t leafai/studio-go-runner-build:$GIT_BRANCH -
        exit_code=$?
        if [ $exit_code -ne 0 ]; then
            exit $exit_code
        fi
		# Information about safely working with temporary files in shell scripts can be found at
        # https://dev.to/philgibbs/avoiding-temporary-files-in-shell-scripts
        {
            stencil -input Dockerfile_standalone > $working_file
            [[ $? != 0 ]] && ExitWithError "stencil processing of Dockerfile_standalone failed"
        } | tee $working_file > /dev/null
        [[ $? != 0 ]] && ExitWithError "Error writing to $working_file"
		docker build -t leafai/studio-go-runner-standalone-build:$GIT_BRANCH -f $working_file .
        rm -f $working_file
		docker tag leafai/studio-go-runner-standalone-build:$GIT_BRANCH leafai/studio-go-runner-standalone-build
		docker tag leafai/studio-go-runner-standalone-build:$GIT_BRANCH localhost:32000/leafai/studio-go-runner-standalone-build
        exit_code=$?
        if [ $exit_code -ne 0 ]; then
            exit $exit_code
        fi
    travis_time_finish
travis_fold end "build.image"

if [ $exit_code -ne 0 ]; then
    exit $exit_code
fi

# Running build.go inside of a container will result in a compilation, light testing, and release however no docker images
travis_fold start "build"
    travis_time_start
        docker run -e TERM="$TERM" -e LOGXI="$LOGXI" -e LOGXI_FORMAT="$LOGXI_FORMAT" -e GITHUB_TOKEN=$GITHUB_TOKEN -v $GOPATH:/project leafai/studio-go-runner-build:$GIT_BRANCH
        exit_code=$?
        if [ $exit_code -ne 0 ]; then
            exit $exit_code
        fi
    travis_time_finish
travis_fold end "build"

if [ $exit_code -ne 0 ]; then
    exit $exit_code
fi

# Automatically produces images without compilation, or releases when run outside of a container
travis_fold start "image.build"
    travis_time_start
        go run -tags=NO_CUDA ./build.go -image-only -r -dirs cmd
        exit_code=$?
        if [ $exit_code -ne 0 ]; then
            exit $exit_code
        fi
    travis_time_finish
travis_fold end "image.build"

if [ $exit_code -ne 0 ]; then
    exit $exit_code
fi

travis_fold start "image.push"
    travis_time_start
		if docker image inspect leaf-ai/studio-go-runner/runner:$SEMVER 2>/dev/null 1>/dev/null; then
			if type docker 2>/dev/null ; then
                docker login docker.io
				if [ $? -eq 0 ]; then
                    docker tag leaf-ai/studio-go-runner/runner:$SEMVER leafai/studio-go-runner:$SEMVER
                    docker tag leafai/studio-go-runner-dev-base:0.0.0 leafai/studio-go-runner-dev-base:$GIT_BRANCH

					docker push leafai/studio-go-runner:$SEMVER
                    docker push leafai/studio-go-runner-dev-base:0.0.0
                    docker push leafai/studio-go-runner-dev-base:$GIT_BRANCH
                    docker push leafai/studio-go-runner-standalone-build:$GIT_BRANCH
			    fi
			fi
			if type aws 2>/dev/null ; then
				`aws ecr get-login --no-include-email`
				if [ $? -eq 0 ]; then
					account=`aws sts get-caller-identity --output text --query Account`
					if [ $? -eq 0 ]; then
						docker tag leafai/studio-go-runner:$SEMVER $account.dkr.ecr.us-west-2.amazonaws.com/leafai/studio-go-runner/runner:$SEMVER
						docker push $account.dkr.ecr.us-west-2.amazonaws.com/leafai/studio-go-runner/runner:$SEMVER

						docker tag leafai/studio-go-runner-standalone-build:$GIT_BRANCH $account.dkr.ecr.us-west-2.amazonaws.com/leafai/studio-go-runner/standalone-build:$GIT_BRANCH
						docker push $account.dkr.ecr.us-west-2.amazonaws.com/leafai/studio-go-runner/standalone-build:$GIT_BRANCH
					fi
				fi
			fi
			if [ -z ${azure_registry_name+x} ]; then
				:
			else
			    if type az 2>/dev/null; then
					if az acr login --name $azure_registry_name; then
						docker tag leafai/studio-go-runner-standalone-build:$GIT_BRANCH $azure_registry_name.azurecr.io/leafai/studio-go-runner-standalone-build:$GIT_BRANCH
						docker push $azure_registry_name.azurecr.io/leafai/studio-go-runner-standalone-build:$GIT_BRANCH

						docker tag leafai/studio-go-runner:$SEMVER $azure_registry_name.azurecr.io/leafai/studio-go-runner:$SEMVER
						docker push $azure_registry_name.azurecr.io/leafai/studio-go-runner:$SEMVER
					fi
				fi
			fi
		fi
    travis_time_finish
travis_fold end "image.push"

exit 0
