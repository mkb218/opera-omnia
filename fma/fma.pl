#!/usr/bin/perl -w

use strict;

use LWP::Simple;
use LWP::UserAgent;
use JSON;
use HTTP::Request::Common;

use constant JIDS_FILE => "gobs/ids.json";

my %ids;

eval {
    open JIDS, "<", JIDS_FILE or die $!;
    my $buf;
    my $string = "";
    my $bytesread;
    while ($bytesread = read(JIDS, $buf, 1024)) {
        $string .= $buf;
    }
    %ids = %{ from_json($string) };
};

if ($@) {
    warn $@;
}

my $apikey = $ARGV[0];
my $hostport;
if (!defined($ARGV[1])) {
    $hostport = "localhost:9001";
} else {
    $hostport = $ARGV[1];
}

my @sorts = qw(track_id track_title track_date_recorded track_listens track_favorites track_date_created);
my $sort = $sorts[int(rand(@sorts))];
my $baseurl = "http://freemusicarchive.org/api/get/tracks.json?sort_by=$sort&limit=1&sort_dir=desc&page=%s&api_key=$apikey&remix=true";

my $page = scalar(keys %ids);
while (1) {
    my $content = get(sprintf($baseurl, $page));
    if (!defined($content)) {
        warn $!;
    }
    warn "got json";
    my $data;
    eval {
        $data = from_json($content);
    };
    if ($@) {
        warn $@;
        next;
    }
    my $track_id = $data->{dataset}[0]{track_id};
    if (defined($ids{$track_id}) && $ids{$track_id}) {
        $page++;
        next;
    }
    $ids{$track_id} = $data->{dataset}[0]{track_id};
    
    my $track_url = $data->{dataset}[0]{track_url};
    my $download_url = "$track_url/download";
    warn "getting data";
    my $rawdata = getstore($download_url, "fma.tmp");
    print length($rawdata)."\n";
    my $ua = LWP::UserAgent->new;
    my $play = "off";
    if (rand() < 0.08) {
        $play = "on";
    }
    my %args = ( add => "on",
                filetype => "mp3",
                play => $play,
                fma_url => $track_url,
                filedata => ["fma.tmp"],
                add => "on");
    my $worked = $ua->request(POST "http://$hostport:9001/upload", Content => \%args, Content_Type => 'form-data');
    print $worked->code;
   print " ";
#    print $worked->content;
    print $track_id;
    print "\n";
    last;
}

my $jout = to_json(\%ids);
open JIDS, ">", JIDS_FILE or die $!;
print JIDS $jout;
close JIDS;
