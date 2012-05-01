#!/usr/bin/perl -w

use strict;

use LWP::Simple;
use LWP::UserAgent;
use JSON;
use HTTP::Request::Common;

my %ids;

dbmopen(%ids,"ids",0666);

my $apikey = $ARGV[0];

my $baseurl = "http://freemusicarchive.org/api/get/tracks.json?sort_by=track_date_created&limit=1&sort_dir=desc&page=%s&api_key=$apikey";

my $page = 1;
while (1) {
    my $content = get(sprintf($baseurl, $page));
    if (!defined($content)) {
        warn $!;
    }
    my $data = from_json($content);
    my $track_id = $data->{dataset}[0]{track_id};
    if (defined($ids{$track_id}) && $ids{$track_id}) {
        $page++;
        next;
    }
    $ids{$track_id} = 1;
    
    my $track_url = $data->{dataset}[0]{track_url};
    $track_url .= "/download";
    my $rawdata = getstore($track_url, "fma.tmp");
    print length($rawdata)."\n";
    my $ua = LWP::UserAgent->new;
    my %args = ( add => "on",
                filetype => "mp3",
                filedata => ["fma.tmp"]);
    my $worked = $ua->request(POST "http://brainchamber.hydrogenproject.com:9001/upload?add=on&filetype=mp3", Content => \%args, Content_Type => 'form-data');
    print $worked->code;
    print " ";
    print $worked->content;
    print "\n";
    last;
}
dbmclose(%ids);