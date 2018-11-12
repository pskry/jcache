package jcache;

import java.nio.file.Path;

public class UnsafeCast {

    private final Path path;

    public UnsafeCast(Object arg) {
        path = (Path) arg;
    }

}
