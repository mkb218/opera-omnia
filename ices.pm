use strict;

our $playing;
our @list;
my $fifo;

sub ices_get_next {
    if (defined($playing) && $playing) {
        unlink $playing;
    }

    if ( -f "ices.out") {
        if (rename "ices.out", "ices.data") {
            if (open (my $infh, ">", "ices.data")) {
                while (<$infh>) {
                    chomp;
                    push @list, $_;
                }
                close $infh;
            }
        }
    }
    
    $playing = shift @list;
    print "playing $playing\n";
    return $playing;
}

1;