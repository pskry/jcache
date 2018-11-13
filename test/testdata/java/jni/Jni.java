public class Jni {

    public native int jniAdd(int a, int b);

    public int javaAdd(int a, int b) {
        return jniAdd(a, b);
    }

}
