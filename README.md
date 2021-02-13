# bucket-filler

This app is designed to be used with [bucket-stream](https://github.com/LtHummus/bucket-stream). In my case, I had a bunch of videos that I wanted to stream to Twitch, but the container was incorrect. This project is an app designed to be run in AWS Lambda to automatically remux videos in to a container that Twitch likes.

## But Isn't Lambda Bad for video encoding?

Yes. Yes it is. Lucky for me, I am not doing video **encoding** but **remuxing**. This is essentially changing the container the video is stored in without modifying the audio and video streams, which means it's much much faster. Remuxing a 1 hour video takes my lambda function a couple of minutes, which works great in Lambda.

### How do I know if I can get away with remuxing instead of reencoding?

That's a little outside the scope of this readme, but you can use a tool like https://mediaarea.net/en/MediaInfo to inspect the codecs used for audio and video in your file. Check out Twitch's requirements at https://stream.twitch.tv/encoding/. Things to keep in mind are the codec used, the bit rate and the [GOP structure](https://en.wikipedia.org/wiki/Group_of_pictures).

## How to use

All of the configuration is done via environment variables:

| Environment Variable | Description |
|----------------------|-------------|
| `DOWNLOAD_DIR`       | Temp storage for videos during the remux process (will require lots of space, see the Lambda Setup section for more details) |
| `INPUT_BUCKET_NAME`  | Bucket that files should be read from. Any triggers from buckets not this one will be ignored |
| `OUTPUT_BUCKET_NAME` | Where to store the remuxed files. This should NOT be the input bucket |
| `REMUX_LOG_TABLE_NAME` | The app stores an entry in a DynamoDB table for each remux, so we can quickly query how things went. |
| `DELETE_WHEN_DONE` | if set to `true` (case sensitive), we will delete the source S3 object on success |


## Lambda Setup

This lambda will require some special setup for one reason: the AWS Lambda runtime limits you to 512 MB of space, which isn't enough space to deal with larger video files (and unfortunately, doing everything in memory...I wasn't able to get working). Here's a list of all the custom things I needed to get going:

### FFMPEG Layer

The first thing is we need a copy of `ffmpeg` to do the remuxing available. This can be achieved using Lambda Layers: https://docs.aws.amazon.com/lambda/latest/dg/configuration-layers.html

I used John Van Sickle's static `ffmpeg` builds for this, which you can get at https://www.johnvansickle.com/ffmpeg/. Package this up as a Lambda layer and attach it to your function.

### VPC

The biggest pain is that this function needs to run inside a VPC of your very own. We need to do this in order to leverage Amazon EFS for additional space (https://aws.amazon.com/efs/). Setting up and configuring a VPC is outside the scope of this tutorial, but suffice to say that you will need to create a VPC (if you don't have one already), create an EFS, set up an access point, and then set up your Lambda function to use both (and don't forget to add service gateways for S3 and DynamoDB!). The `DOWNLOAD_DIR` environment variable should be the mount point of your EFS volume.

### S3 Trigger

You will probably want to set up a Lambda trigger to fire when an object is uploaded to S3. Just keep in mind you shouldn't trigger this lambda on the output bucket lest you create some infinite recursion of remuxing.
