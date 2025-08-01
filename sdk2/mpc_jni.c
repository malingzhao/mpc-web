#include <jni.h>
#include <string.h>
#include <stdlib.h>
#include "libmpc.h"

// ==================== 密钥生成 (Key Generation) ====================

JNIEXPORT jlong JNICALL
Java_com_example_mpctest_MPCNative_keygenInit(JNIEnv *env, jclass clazz, 
                                               jint curve, jint partyID, jint threshold, jint totalParties) {
    void* handle = NULL;
    int result = go_keygen_init(curve, partyID, threshold, totalParties, &handle);
    if (result != 0) {
        return 0; // 返回NULL指针表示失败
    }
    return (jlong)handle;
}

JNIEXPORT jbyteArray JNICALL
Java_com_example_mpctest_MPCNative_keygenRound1(JNIEnv *env, jclass clazz, jlong handle) {
    char* outData = NULL;
    int outLen = 0;
    
    int result = go_keygen_round1((void*)handle, &outData, &outLen);
    if (result != 0 || outData == NULL) {
        return NULL;
    }
    
    jbyteArray jdata = (*env)->NewByteArray(env, outLen);
    (*env)->SetByteArrayRegion(env, jdata, 0, outLen, (jbyte*)outData);
    
    // 释放Go分配的内存
    mpc_string_free(outData);
    
    return jdata;
}

JNIEXPORT jbyteArray JNICALL
Java_com_example_mpctest_MPCNative_keygenRound2(JNIEnv *env, jclass clazz, 
                                                 jlong handle, jbyteArray inData) {
    jbyte* inBytes = (*env)->GetByteArrayElements(env, inData, NULL);
    jsize inLen = (*env)->GetArrayLength(env, inData);
    
    char* outData = NULL;
    int outLen = 0;
    
    int result = go_keygen_round2((void*)handle, (char*)inBytes, inLen, &outData, &outLen);
    
    (*env)->ReleaseByteArrayElements(env, inData, inBytes, JNI_ABORT);
    
    if (result != 0 || outData == NULL) {
        return NULL;
    }
    
    jbyteArray jdata = (*env)->NewByteArray(env, outLen);
    (*env)->SetByteArrayRegion(env, jdata, 0, outLen, (jbyte*)outData);
    
    mpc_string_free(outData);
    
    return jdata;
}

JNIEXPORT jbyteArray JNICALL
Java_com_example_mpctest_MPCNative_keygenRound3(JNIEnv *env, jclass clazz, 
                                                 jlong handle, jbyteArray inData) {
    jbyte* inBytes = (*env)->GetByteArrayElements(env, inData, NULL);
    jsize inLen = (*env)->GetArrayLength(env, inData);
    
    char* keyData = NULL;
    int keyLen = 0;
    
    int result = go_keygen_round3((void*)handle, (char*)inBytes, inLen, &keyData, &keyLen);
    
    (*env)->ReleaseByteArrayElements(env, inData, inBytes, JNI_ABORT);
    
    if (result != 0 || keyData == NULL) {
        return NULL;
    }
    
    jbyteArray jdata = (*env)->NewByteArray(env, keyLen);
    (*env)->SetByteArrayRegion(env, jdata, 0, keyLen, (jbyte*)keyData);
    
    mpc_string_free(keyData);
    
    return jdata;
}

JNIEXPORT void JNICALL
Java_com_example_mpctest_MPCNative_keygenDestroy(JNIEnv *env, jclass clazz, jlong handle) {
    go_keygen_destroy((void*)handle);
}

// ==================== 密钥刷新 (Key Refresh) ====================

JNIEXPORT jlong JNICALL
Java_com_example_mpctest_MPCNative_refreshInit(JNIEnv *env, jclass clazz, 
                                                jint curve, jint partyID, jint threshold, 
                                                jintArray devoteList, jbyteArray keyData) {
    jint* devoteArray = (*env)->GetIntArrayElements(env, devoteList, NULL);
    jsize devoteCount = (*env)->GetArrayLength(env, devoteList);
    
    jbyte* keyBytes = (*env)->GetByteArrayElements(env, keyData, NULL);
    jsize keyLen = (*env)->GetArrayLength(env, keyData);
    
    void* handle = NULL;
    int result = go_refresh_init(curve, partyID, threshold, (int*)devoteArray, devoteCount, 
                                (char*)keyBytes, keyLen, &handle);
    
    (*env)->ReleaseIntArrayElements(env, devoteList, devoteArray, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, keyData, keyBytes, JNI_ABORT);
    
    if (result != 0) {
        return 0;
    }
    return (jlong)handle;
}

JNIEXPORT jbyteArray JNICALL
Java_com_example_mpctest_MPCNative_refreshRound1(JNIEnv *env, jclass clazz, jlong handle) {
    char* outData = NULL;
    int outLen = 0;
    
    int result = go_refresh_round1((void*)handle, &outData, &outLen);
    if (result != 0 || outData == NULL) {
        return NULL;
    }
    
    jbyteArray jdata = (*env)->NewByteArray(env, outLen);
    (*env)->SetByteArrayRegion(env, jdata, 0, outLen, (jbyte*)outData);
    
    mpc_string_free(outData);
    
    return jdata;
}

JNIEXPORT jbyteArray JNICALL
Java_com_example_mpctest_MPCNative_refreshRound2(JNIEnv *env, jclass clazz, 
                                                  jlong handle, jbyteArray inData) {
    jbyte* inBytes = (*env)->GetByteArrayElements(env, inData, NULL);
    jsize inLen = (*env)->GetArrayLength(env, inData);
    
    char* outData = NULL;
    int outLen = 0;
    
    int result = go_refresh_round2((void*)handle, (char*)inBytes, inLen, &outData, &outLen);
    
    (*env)->ReleaseByteArrayElements(env, inData, inBytes, JNI_ABORT);
    
    if (result != 0 || outData == NULL) {
        return NULL;
    }
    
    jbyteArray jdata = (*env)->NewByteArray(env, outLen);
    (*env)->SetByteArrayRegion(env, jdata, 0, outLen, (jbyte*)outData);
    
    mpc_string_free(outData);
    
    return jdata;
}

JNIEXPORT jbyteArray JNICALL
Java_com_example_mpctest_MPCNative_refreshRound3(JNIEnv *env, jclass clazz, 
                                                  jlong handle, jbyteArray inData) {
    jbyte* inBytes = (*env)->GetByteArrayElements(env, inData, NULL);
    jsize inLen = (*env)->GetArrayLength(env, inData);
    
    char* keyData = NULL;
    int keyLen = 0;
    
    int result = go_refresh_round3((void*)handle, (char*)inBytes, inLen, &keyData, &keyLen);
    
    (*env)->ReleaseByteArrayElements(env, inData, inBytes, JNI_ABORT);
    
    if (result != 0 || keyData == NULL) {
        return NULL;
    }
    
    jbyteArray jdata = (*env)->NewByteArray(env, keyLen);
    (*env)->SetByteArrayRegion(env, jdata, 0, keyLen, (jbyte*)keyData);
    
    mpc_string_free(keyData);
    
    return jdata;
}

JNIEXPORT void JNICALL
Java_com_example_mpctest_MPCNative_refreshDestroy(JNIEnv *env, jclass clazz, jlong handle) {
    go_refresh_destroy((void*)handle);
}

// ==================== Ed25519签名 ====================

JNIEXPORT jlong JNICALL
Java_com_example_mpctest_MPCNative_ed25519SignInit(JNIEnv *env, jclass clazz, 
                                                    jint partyID, jint threshold, jintArray partList,
                                                    jbyteArray keyData, jbyteArray message) {
    jint* partArray = (*env)->GetIntArrayElements(env, partList, NULL);
    jsize partCount = (*env)->GetArrayLength(env, partList);
    
    jbyte* keyBytes = (*env)->GetByteArrayElements(env, keyData, NULL);
    jsize keyLen = (*env)->GetArrayLength(env, keyData);
    
    jbyte* msgBytes = (*env)->GetByteArrayElements(env, message, NULL);
    jsize msgLen = (*env)->GetArrayLength(env, message);
    
    void* handle = NULL;
    int result = go_ed25519_sign_init(partyID, threshold, (int*)partArray, partCount,
                                     (char*)keyBytes, keyLen, (char*)msgBytes, msgLen, &handle);
    
    (*env)->ReleaseIntArrayElements(env, partList, partArray, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, keyData, keyBytes, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, message, msgBytes, JNI_ABORT);
    
    if (result != 0) {
        return 0;
    }
    return (jlong)handle;
}

JNIEXPORT jbyteArray JNICALL
Java_com_example_mpctest_MPCNative_ed25519SignRound1(JNIEnv *env, jclass clazz, jlong handle) {
    char* outData = NULL;
    int outLen = 0;
    
    int result = go_ed25519_sign_round1((void*)handle, &outData, &outLen);
    if (result != 0 || outData == NULL) {
        return NULL;
    }
    
    jbyteArray jdata = (*env)->NewByteArray(env, outLen);
    (*env)->SetByteArrayRegion(env, jdata, 0, outLen, (jbyte*)outData);
    
    mpc_string_free(outData);
    
    return jdata;
}

JNIEXPORT jbyteArray JNICALL
Java_com_example_mpctest_MPCNative_ed25519SignRound2(JNIEnv *env, jclass clazz, 
                                                      jlong handle, jbyteArray inData) {
    jbyte* inBytes = (*env)->GetByteArrayElements(env, inData, NULL);
    jsize inLen = (*env)->GetArrayLength(env, inData);
    
    char* outData = NULL;
    int outLen = 0;
    
    int result = go_ed25519_sign_round2((void*)handle, (char*)inBytes, inLen, &outData, &outLen);
    
    (*env)->ReleaseByteArrayElements(env, inData, inBytes, JNI_ABORT);
    
    if (result != 0 || outData == NULL) {
        return NULL;
    }
    
    jbyteArray jdata = (*env)->NewByteArray(env, outLen);
    (*env)->SetByteArrayRegion(env, jdata, 0, outLen, (jbyte*)outData);
    
    mpc_string_free(outData);
    
    return jdata;
}

JNIEXPORT jobjectArray JNICALL
Java_com_example_mpctest_MPCNative_ed25519SignRound3(JNIEnv *env, jclass clazz, 
                                                      jlong handle, jbyteArray inData) {
    jbyte* inBytes = (*env)->GetByteArrayElements(env, inData, NULL);
    jsize inLen = (*env)->GetArrayLength(env, inData);
    
    char* sigR = NULL;
    char* sigS = NULL;
    
    int result = go_ed25519_sign_round3((void*)handle, (char*)inBytes, inLen, &sigR, &sigS);
    
    (*env)->ReleaseByteArrayElements(env, inData, inBytes, JNI_ABORT);
    
    if (result != 0 || sigR == NULL || sigS == NULL) {
        return NULL;
    }
    
    // 创建String数组
    jclass stringClass = (*env)->FindClass(env, "java/lang/String");
    jobjectArray result_array = (*env)->NewObjectArray(env, 2, stringClass, NULL);
    
    jstring jR = (*env)->NewStringUTF(env, sigR);
    jstring jS = (*env)->NewStringUTF(env, sigS);
    
    (*env)->SetObjectArrayElement(env, result_array, 0, jR);
    (*env)->SetObjectArrayElement(env, result_array, 1, jS);
    
    mpc_string_free(sigR);
    mpc_string_free(sigS);
    
    return result_array;
}

JNIEXPORT void JNICALL
Java_com_example_mpctest_MPCNative_ed25519SignDestroy(JNIEnv *env, jclass clazz, jlong handle) {
    go_ed25519_sign_destroy((void*)handle);
}

// ==================== ECDSA Keygen ====================

JNIEXPORT jbyteArray JNICALL Java_com_example_mpctest_MPCNative_ecdsaKeygenGenerateP2Params(JNIEnv *env, jclass cls) {
    char* out_data = NULL;
    int out_len = 0;
    
    int result = go_ecdsa_keygen_generate_p2_params(&out_data, &out_len);
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "ECDSA P2参数生成失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return NULL;
    }
    
    jbyteArray result_array = (*env)->NewByteArray(env, out_len);
    (*env)->SetByteArrayRegion(env, result_array, 0, out_len, (jbyte*)out_data);
    
    if (out_data) free(out_data);
    return result_array;
}

JNIEXPORT jobjectArray JNICALL Java_com_example_mpctest_MPCNative_ecdsaKeygenP1(JNIEnv *env, jclass cls, jbyteArray keyData, jint peerId, jbyteArray p2Params) {
    jbyte* key_bytes = (*env)->GetByteArrayElements(env, keyData, NULL);
    jsize key_len = (*env)->GetArrayLength(env, keyData);
    
    jbyte* p2_bytes = (*env)->GetByteArrayElements(env, p2Params, NULL);
    jsize p2_len = (*env)->GetArrayLength(env, p2Params);
    
    char* out_data = NULL;
    int out_len = 0;
    char* message_data = NULL;
    int message_len = 0;
    
    int result = go_ecdsa_keygen_p1((char*)key_bytes, key_len, peerId, 
                                   (char*)p2_bytes, p2_len,
                                   &out_data, &out_len,
                                   &message_data, &message_len);
    
    (*env)->ReleaseByteArrayElements(env, keyData, key_bytes, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, p2Params, p2_bytes, JNI_ABORT);
    
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "P1 ECDSA keygen失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return NULL;
    }
    
    // 创建返回数组 [signData, messageData]
    jclass byteArrayClass = (*env)->FindClass(env, "[B");
    jobjectArray result_array = (*env)->NewObjectArray(env, 2, byteArrayClass, NULL);
    
    jbyteArray sign_array = (*env)->NewByteArray(env, out_len);
    (*env)->SetByteArrayRegion(env, sign_array, 0, out_len, (jbyte*)out_data);
    (*env)->SetObjectArrayElement(env, result_array, 0, sign_array);
    
    jbyteArray message_array = (*env)->NewByteArray(env, message_len);
    (*env)->SetByteArrayRegion(env, message_array, 0, message_len, (jbyte*)message_data);
    (*env)->SetObjectArrayElement(env, result_array, 1, message_array);
    
    if (out_data) free(out_data);
    if (message_data) free(message_data);
    return result_array;
}

JNIEXPORT jbyteArray JNICALL Java_com_example_mpctest_MPCNative_ecdsaKeygenP2(JNIEnv *env, jclass cls, jbyteArray keyData, jint p1Id, jbyteArray p1Message, jbyteArray p2Params) {
    jbyte* key_bytes = (*env)->GetByteArrayElements(env, keyData, NULL);
    jsize key_len = (*env)->GetArrayLength(env, keyData);
    
    jbyte* p1_msg_bytes = (*env)->GetByteArrayElements(env, p1Message, NULL);
    jsize p1_msg_len = (*env)->GetArrayLength(env, p1Message);
    
    jbyte* p2_bytes = (*env)->GetByteArrayElements(env, p2Params, NULL);
    jsize p2_len = (*env)->GetArrayLength(env, p2Params);
    
    char* out_data = NULL;
    int out_len = 0;
    
    int result = go_ecdsa_keygen_p2((char*)key_bytes, key_len, p1Id,
                                   (char*)p1_msg_bytes, p1_msg_len,
                                   (char*)p2_bytes, p2_len,
                                   &out_data, &out_len);
    
    (*env)->ReleaseByteArrayElements(env, keyData, key_bytes, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, p1Message, p1_msg_bytes, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, p2Params, p2_bytes, JNI_ABORT);
    
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "P2 ECDSA keygen失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return NULL;
    }
    
    jbyteArray result_array = (*env)->NewByteArray(env, out_len);
    (*env)->SetByteArrayRegion(env, result_array, 0, out_len, (jbyte*)out_data);
    
    if (out_data) free(out_data);
    return result_array;
}

// ==================== ECDSA签名 ====================

JNIEXPORT jlong JNICALL Java_com_example_mpctest_MPCNative_ecdsaSignInitP1Complex(JNIEnv *env, jclass cls, jint partyID, jint peerID, jbyteArray keyData, jbyteArray message) {
    jbyte* key_bytes = (*env)->GetByteArrayElements(env, keyData, NULL);
    jsize key_len = (*env)->GetArrayLength(env, keyData);
    
    jbyte* msg_bytes = (*env)->GetByteArrayElements(env, message, NULL);
    jsize msg_len = (*env)->GetArrayLength(env, message);
    
    void* handle = NULL;
    int result = go_ecdsa_sign_init_p1_complex(partyID, peerID, 
                                              (char*)key_bytes, key_len,
                                              (char*)msg_bytes, msg_len,
                                              &handle);
    
    (*env)->ReleaseByteArrayElements(env, keyData, key_bytes, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, message, msg_bytes, JNI_ABORT);
    
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "P1 ECDSA签名初始化失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return 0;
    }
    
    return (jlong)handle;
}

JNIEXPORT jlong JNICALL Java_com_example_mpctest_MPCNative_ecdsaSignInitP2Complex(JNIEnv *env, jclass cls, jint partyID, jint peerID, jbyteArray keyData, jbyteArray message) {
    jbyte* key_bytes = (*env)->GetByteArrayElements(env, keyData, NULL);
    jsize key_len = (*env)->GetArrayLength(env, keyData);
    
    jbyte* msg_bytes = (*env)->GetByteArrayElements(env, message, NULL);
    jsize msg_len = (*env)->GetArrayLength(env, message);
    
    void* handle = NULL;
    int result = go_ecdsa_sign_init_p2_complex(partyID, peerID,
                                              (char*)key_bytes, key_len,
                                              (char*)msg_bytes, msg_len,
                                              &handle);
    
    (*env)->ReleaseByteArrayElements(env, keyData, key_bytes, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, message, msg_bytes, JNI_ABORT);
    
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "P2 ECDSA签名初始化失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return 0;
    }
    
    return (jlong)handle;
}

JNIEXPORT jbyteArray JNICALL Java_com_example_mpctest_MPCNative_ecdsaSignStep1(JNIEnv *env, jclass cls, jlong handle) {
    char* out_data = NULL;
    int out_len = 0;
    
    int result = go_ecdsa_sign_step1((void*)handle, &out_data, &out_len);
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "ECDSA签名Step1失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return NULL;
    }
    
    jbyteArray result_array = (*env)->NewByteArray(env, out_len);
    (*env)->SetByteArrayRegion(env, result_array, 0, out_len, (jbyte*)out_data);
    
    if (out_data) free(out_data);
    return result_array;
}

JNIEXPORT jobjectArray JNICALL Java_com_example_mpctest_MPCNative_ecdsaSignP2Step1(JNIEnv *env, jclass cls, jlong handle, jbyteArray commitData) {
    jbyte* commit_bytes = (*env)->GetByteArrayElements(env, commitData, NULL);
    jsize commit_len = (*env)->GetArrayLength(env, commitData);
    
    char* proof_data = NULL;
    int proof_len = 0;
    char* r2_data = NULL;
    int r2_len = 0;
    
    int result = go_ecdsa_sign_p2_step1((void*)handle, (char*)commit_bytes, commit_len,
                                       &proof_data, &proof_len,
                                       &r2_data, &r2_len);
    
    (*env)->ReleaseByteArrayElements(env, commitData, commit_bytes, JNI_ABORT);
    
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "ECDSA签名P2Step1失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return NULL;
    }
    
    // 创建返回数组
    jclass byteArrayClass = (*env)->FindClass(env, "[B");
    jobjectArray result_array = (*env)->NewObjectArray(env, 2, byteArrayClass, NULL);
    
    jbyteArray proof_array = (*env)->NewByteArray(env, proof_len);
    (*env)->SetByteArrayRegion(env, proof_array, 0, proof_len, (jbyte*)proof_data);
    (*env)->SetObjectArrayElement(env, result_array, 0, proof_array);
    
    jbyteArray r2_array = (*env)->NewByteArray(env, r2_len);
    (*env)->SetByteArrayRegion(env, r2_array, 0, r2_len, (jbyte*)r2_data);
    (*env)->SetObjectArrayElement(env, result_array, 1, r2_array);
    
    if (proof_data) free(proof_data);
    if (r2_data) free(r2_data);
    return result_array;
}

JNIEXPORT jobjectArray JNICALL Java_com_example_mpctest_MPCNative_ecdsaSignP1Step2(JNIEnv *env, jclass cls, jlong handle, jbyteArray proofData, jbyteArray r2Data) {
    jbyte* proof_bytes = (*env)->GetByteArrayElements(env, proofData, NULL);
    jsize proof_len = (*env)->GetArrayLength(env, proofData);
    
    jbyte* r2_bytes = (*env)->GetByteArrayElements(env, r2Data, NULL);
    jsize r2_len = (*env)->GetArrayLength(env, r2Data);
    
    char* p1_proof_data = NULL;
    int p1_proof_len = 0;
    char* cmtd_data = NULL;
    int cmtd_len = 0;
    
    int result = go_ecdsa_sign_p1_step2((void*)handle,
                                       (char*)proof_bytes, proof_len,
                                       (char*)r2_bytes, r2_len,
                                       &p1_proof_data, &p1_proof_len,
                                       &cmtd_data, &cmtd_len);
    
    (*env)->ReleaseByteArrayElements(env, proofData, proof_bytes, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, r2Data, r2_bytes, JNI_ABORT);
    
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "ECDSA签名P1Step2失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return NULL;
    }
    
    // 创建返回数组
    jclass byteArrayClass = (*env)->FindClass(env, "[B");
    jobjectArray result_array = (*env)->NewObjectArray(env, 2, byteArrayClass, NULL);
    
    jbyteArray p1_proof_array = (*env)->NewByteArray(env, p1_proof_len);
    (*env)->SetByteArrayRegion(env, p1_proof_array, 0, p1_proof_len, (jbyte*)p1_proof_data);
    (*env)->SetObjectArrayElement(env, result_array, 0, p1_proof_array);
    
    jbyteArray cmtd_array = (*env)->NewByteArray(env, cmtd_len);
    (*env)->SetByteArrayRegion(env, cmtd_array, 0, cmtd_len, (jbyte*)cmtd_data);
    (*env)->SetObjectArrayElement(env, result_array, 1, cmtd_array);
    
    if (p1_proof_data) free(p1_proof_data);
    if (cmtd_data) free(cmtd_data);
    return result_array;
}

JNIEXPORT jobjectArray JNICALL Java_com_example_mpctest_MPCNative_ecdsaSignP2Step2(JNIEnv *env, jclass cls, jlong handle, jbyteArray cmtdData, jbyteArray p1ProofData) {
    jbyte* cmtd_bytes = (*env)->GetByteArrayElements(env, cmtdData, NULL);
    jsize cmtd_len = (*env)->GetArrayLength(env, cmtdData);
    
    jbyte* p1_proof_bytes = (*env)->GetByteArrayElements(env, p1ProofData, NULL);
    jsize p1_proof_len = (*env)->GetArrayLength(env, p1ProofData);
    
    char* ek_data = NULL;
    int ek_len = 0;
    char* affine_proof_data = NULL;
    int affine_proof_len = 0;
    
    int result = go_ecdsa_sign_p2_step2((void*)handle,
                                       (char*)cmtd_bytes, cmtd_len,
                                       (char*)p1_proof_bytes, p1_proof_len,
                                       &ek_data, &ek_len,
                                       &affine_proof_data, &affine_proof_len);
    
    (*env)->ReleaseByteArrayElements(env, cmtdData, cmtd_bytes, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, p1ProofData, p1_proof_bytes, JNI_ABORT);
    
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "ECDSA签名P2Step2失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return NULL;
    }
    
    // 创建返回数组
    jclass byteArrayClass = (*env)->FindClass(env, "[B");
    jobjectArray result_array = (*env)->NewObjectArray(env, 2, byteArrayClass, NULL);
    
    jbyteArray ek_array = (*env)->NewByteArray(env, ek_len);
    (*env)->SetByteArrayRegion(env, ek_array, 0, ek_len, (jbyte*)ek_data);
    (*env)->SetObjectArrayElement(env, result_array, 0, ek_array);
    
    jbyteArray affine_array = (*env)->NewByteArray(env, affine_proof_len);
    (*env)->SetByteArrayRegion(env, affine_array, 0, affine_proof_len, (jbyte*)affine_proof_data);
    (*env)->SetObjectArrayElement(env, result_array, 1, affine_array);
    
    if (ek_data) free(ek_data);
    if (affine_proof_data) free(affine_proof_data);
    return result_array;
}

JNIEXPORT jobjectArray JNICALL Java_com_example_mpctest_MPCNative_ecdsaSignP1Step3(JNIEnv *env, jclass cls, jlong handle, jbyteArray ekData, jbyteArray affineProofData) {
    jbyte* ek_bytes = (*env)->GetByteArrayElements(env, ekData, NULL);
    jsize ek_len = (*env)->GetArrayLength(env, ekData);
    
    jbyte* affine_bytes = (*env)->GetByteArrayElements(env, affineProofData, NULL);
    jsize affine_len = (*env)->GetArrayLength(env, affineProofData);
    
    char* signature_r = NULL;
    int signature_r_len = 0;
    char* signature_s = NULL;
    int signature_s_len = 0;
    
    int result = go_ecdsa_sign_p1_step3((void*)handle,
                                       (char*)ek_bytes, ek_len,
                                       (char*)affine_bytes, affine_len,
                                       &signature_r, &signature_r_len,
                                       &signature_s, &signature_s_len);
    
    (*env)->ReleaseByteArrayElements(env, ekData, ek_bytes, JNI_ABORT);
    (*env)->ReleaseByteArrayElements(env, affineProofData, affine_bytes, JNI_ABORT);
    
    if (result != 0) {
        char error_msg[256];
        snprintf(error_msg, sizeof(error_msg), "ECDSA签名P1Step3失败: %d", result);
        jclass exception_class = (*env)->FindClass(env, "java/lang/RuntimeException");
        (*env)->ThrowNew(env, exception_class, error_msg);
        return NULL;
    }
    
    // 创建返回数组
    jclass stringClass = (*env)->FindClass(env, "java/lang/String");
    jobjectArray result_array = (*env)->NewObjectArray(env, 2, stringClass, NULL);
    
    jstring r_string = (*env)->NewStringUTF(env, signature_r);
    (*env)->SetObjectArrayElement(env, result_array, 0, r_string);
    
    jstring s_string = (*env)->NewStringUTF(env, signature_s);
    (*env)->SetObjectArrayElement(env, result_array, 1, s_string);
    
    if (signature_r) free(signature_r);
    if (signature_s) free(signature_s);
    return result_array;
}

JNIEXPORT void JNICALL Java_com_example_mpctest_MPCNative_ecdsaSignDestroy(JNIEnv *env, jclass cls, jlong handle) {
    if (handle != 0) {
        go_ecdsa_sign_destroy((void*)handle);
    }
}

// ==================== 辅助函数 ====================

JNIEXPORT jstring JNICALL
Java_com_example_mpctest_MPCNative_getErrorString(JNIEnv *env, jclass clazz, jint errorCode) {
    char* errorStr = mpc_get_error_string(errorCode);
    if (errorStr == NULL) {
        return NULL;
    }
    
    jstring result = (*env)->NewStringUTF(env, errorStr);
    mpc_string_free(errorStr);
    
    return result;
}

JNIEXPORT jlong JNICALL
Java_com_example_mpctest_MPCNative_allocString(JNIEnv *env, jclass clazz, jstring src) {
    const char* cStr = (*env)->GetStringUTFChars(env, src, NULL);
    char* allocated = mpc_string_alloc((char*)cStr);
    (*env)->ReleaseStringUTFChars(env, src, cStr);
    
    return (jlong)allocated;
}

JNIEXPORT void JNICALL
Java_com_example_mpctest_MPCNative_freeString(JNIEnv *env, jclass clazz, jlong ptr) {
    mpc_string_free((char*)ptr);
}